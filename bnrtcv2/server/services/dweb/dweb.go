package dweb

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/guabee/bnrtc/bnrtcv2/client"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/bnrtcv2/server/iservice"
	"github.com/guabee/bnrtc/bnrtcv2/server/services"

	conf "github.com/guabee/bnrtc/bnrtcv2/config"
	"github.com/guabee/bnrtc/util"
)

const DwebServiceName = "dweb"

var (
	HttpPort       = 19233
	DPort          = "DWEB"
	RequestTimeout = 60 * 2 * time.Second
)

type DwebConfig struct {
	Port int `json:"port"`
}

type DwebServiceFactory struct {
	once    sync.Once
	service *Service
}

func (f *DwebServiceFactory) LoadOptions(options conf.OptionMap) {
	// services.dweb.httpPort
	httpPort := options.GetInt("http_port")
	if httpPort > 0 {
		HttpPort = httpPort
	}

	// services.dweb.dport
	dport := options.GetString("dport")
	if dport != "" {
		DPort = dport
	}

	// services.dweb.requestTimeout
	requestTimeout := options.GetDuration("request_timeout")
	if requestTimeout > 0 {
		RequestTimeout = requestTimeout
	}
}

func (f *DwebServiceFactory) GetInstance(server iservice.IServer, options conf.OptionMap) (iservice.IService, error) {
	f.once.Do(func() {
		f.LoadOptions(options)
		channel := server.NewLocalChannel(DwebServiceName)
		f.service = NewDwebServer(channel, WithHttpPort(HttpPort), WithDport(DPort), WithTimeout(RequestTimeout))
		_ = server.AddServicesHandler(DwebServiceName, "config", func(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
			port := DwebConfig{
				Port: f.service.HttpPort,
			}
			return json.Marshal(port)
		})
	})

	return f.service, nil
}

func (f *DwebServiceFactory) Name() string {
	return DwebServiceName
}

func init() {
	services.RegisterServiceFactory(&DwebServiceFactory{})
}

type HttpProxyRequest struct {
	ReqId    uint32              `json:"reqId"`
	Path     string              `json:"path"`
	Method   string              `json:"method"`
	Header   map[string][]string `json:"header"`
	PostForm map[string][]string `json:"postForm"`
}
type HttpProxyResponse struct {
	ReqId      uint32            `json:"reqId"`
	StatusCode uint16            `json:"statusCode"`
	Header     map[string]string `json:"header"`
	Data       []byte            `json:"data"`
}
type DwebWsMessage struct {
	DPort string `json:"dPort"`
	Url   string `json:"url"`
	Type  string `json:"type"`
}

type IBnrtcClient interface {
	Send(src string, dst string, dport string, devid string, data []byte) error
	OnMessage(dport string, handler client.HandlerFunc) error
	OffMessage(dport string, handler client.HandlerFunc) error
	Close()
}

type Service struct {
	BnrtcClient      IBnrtcClient
	HttpPort         int
	RequestTimeout   time.Duration
	DPort            string
	HttpServer       *http.Server
	notFoundPageByte []byte
	upgrader         websocket.Upgrader
	dPortWsMap       sync.Map
	reqRespManager   *util.ReqRespManager
	logger           *log.Entry
}

type Option func(option *Service)

func WithHttpPort(port int) Option {
	return func(s *Service) {
		if port > 0 && port < 65536 {
			s.HttpPort = port
		}
	}
}

func WithDport(dport string) Option {
	return func(s *Service) {
		s.DPort = dport
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(s *Service) {
		if timeout > 0 {
			s.RequestTimeout = timeout
		}
	}
}

func NewDwebServer(client IBnrtcClient, options ...Option) *Service {
	s := &Service{
		BnrtcClient:    client,
		HttpPort:       HttpPort,
		RequestTimeout: RequestTimeout,
		DPort:          DPort,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	for _, option := range options {
		option(s)
	}

	s.reqRespManager = util.NewReqRespManager(util.WithReqRespTimeout(s.RequestTimeout))
	s.HttpServer = &http.Server{Addr: "127.0.0.1:" + strconv.Itoa(int(s.HttpPort))}
	s.logger = log.NewLoggerEntry(DwebServiceName).WithField("port", s.HttpPort)

	return s
}

func (f *Service) Name() string {
	return DwebServiceName
}

func (s *Service) Start() error {
	if s.BnrtcClient == nil {
		return errors.New("BnrtcClient not exsist")
	}
	err := s.BnrtcClient.OnMessage(s.DPort, s.bnrtcMessageHandler)
	if err != nil {
		return err
	}
	handler := http.HandlerFunc(s.httpServerHandler)
	s.HttpServer.Handler = handler
	s.logger.Infof("start dweb http server, %s(%d)", s.DPort, s.HttpPort)
	s.reqRespManager.Start()
	return s.HttpServer.ListenAndServe()
}

func (s *Service) Stop() error {
	err := s.BnrtcClient.OffMessage(s.DPort, s.bnrtcMessageHandler)
	if err != nil {
		return err
	}
	s.BnrtcClient.Close()
	s.reqRespManager.Close()
	return s.HttpServer.Close()
}

func (s *Service) BindAddress(address string) {
	// no need implement
}

func (s *Service) bnrtcMessageHandler(msg *message.Message) {
	s.logger.Debugf("bnrtcMessageHandler, src: %v, dst: %v, DPort: %v, dataLength: %v", msg.SrcAddr, msg.DstAddr, msg.Dport, len(msg.Buffer.Data()))
	res := HttpProxyResponse{}
	err := decodeDwebResponce(msg.Buffer.Data(), &res)
	if err != nil {
		s.logger.Errorf("decode DwebResponce, %v, %v", msg, err)
		return
	}

	s.reqRespManager.ResolveReq(res.ReqId, &res)
}

func ValidDwebHost(host string) (string, string, error) {
	regl := regexp.MustCompile(`^([\w\.]+)\.localhost`)
	valid := regl.Match([]byte(host))
	if !valid {
		return "", "", fmt.Errorf("can not valid")
	}
	regResult := regl.FindStringSubmatch(host)
	hexUrl := ""
	for _, v := range regResult[1:] {
		hexUrl = hexUrl + v
	}
	hexUrl = strings.Replace(hexUrl, ".", "", -1)
	url, err := hex.DecodeString(hexUrl)
	if err != nil {
		return "", "", err
	}
	{
		regl := regexp.MustCompile(`^(\w+):(.+)`)
		regResult := regl.FindStringSubmatch(string(url))
		if len(regResult) != 3 {
			return "", "", fmt.Errorf("can not valid")
		}
		return regResult[1], regResult[2], nil
	}
}

func (s *Service) httpServerHandler(rw http.ResponseWriter, r *http.Request) {
	/// default allow cors
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("Access-Control-Allow-Method", "*")
	rw.Header().Set("Access-Control-Allow-Headers", "*")
	rw.Header().Set("Access-Control-Expose-Headers", "Accept-Ranges")
	if r.Method == "OPTIONS" {
		rw.WriteHeader(200)
		return
	}
	{
		/// proxy bnrtc request
		address, host, validErr := ValidDwebHost(r.Host)
		if validErr == nil {
			s.proxyBnrtc2Request(address, host, rw, r)
			return
		}
	}
	{
		/// dweb server handler
		handlerErr := s.handlerDwebServer(rw, r)
		if handlerErr == nil {
			return
		}
	}
	rw.WriteHeader(404)
}

func (s *Service) handlerDwebServer(
	rw http.ResponseWriter,
	r *http.Request,
) error {

	switch r.URL.Path {
	case "/customNotFoundPage":
		body, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			rw.WriteHeader(500)
			_, err = rw.Write([]byte(err.Error()))
			if err != nil {
				return err
			}
		} else {
			s.notFoundPageByte = body
		}
		return nil
	// case "/test":
	// 	downloadErr := s.requestDownload("", "", rw, r)
	// 	if downloadErr == nil {
	// 		return nil
	// 	}
	// 	rw.Header().Add("content-type", "application/octet-stream")
	// 	rw.WriteHeader(200)
	// 	if r.Method == "HEAD" {
	// 		return nil
	// 	}
	// 	writer := bufio.NewWriter(rw)
	// 	buf := make([]byte, 1024*1024)
	// 	i := 0
	// 	for {
	// 		i++
	// 		_, err := writer.Write(buf)
	// 		if err != nil {
	// 			break
	// 		}
	// 		feer := writer.Flush()
	// 		if feer != nil {
	// 			break
	// 		}
	// 		if i == 10 {
	// 			break
	// 		}
	// 		<-time.After(time.Duration(2) * time.Second)
	// 	}
	// 	return nil
	case "/connectWs":
		dport := r.URL.Query().Get("dPort")
		if dport == "" {
			return nil
		}
		s.upgradeWs(dport, rw, r)
		return nil
	}
	return fmt.Errorf("can not handler because not match path")
}

func (s *Service) upgradeWs(
	dport string,
	rw http.ResponseWriter,
	r *http.Request,
) {
	ws, err := s.upgrader.Upgrade(rw, r, nil)
	if err != nil {
		return
	}
	wsMap, _ := s.dPortWsMap.LoadOrStore(dport, make(map[string]*websocket.Conn))
	wsAddress := time.Now().UTC().String()
	wsMap.(map[string]*websocket.Conn)[wsAddress] = ws
	defer ws.Close()
	defer func() {
		delete(wsMap.(map[string]*websocket.Conn), wsAddress)
	}()
	for {
		//读取ws中的数据
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (s *Service) requestDownload(
	address string,
	dPort string,
	rw http.ResponseWriter,
	r *http.Request,
) error {
	secFetchMode := r.Header.Get("sec-fetch-mode")
	secFetchUser := r.Header.Get("Sec-Fetch-User")
	if secFetchMode != "navigate" && secFetchUser != "?1" {
		return fmt.Errorf("can not download handler")
	}
	headRes, err := s.bnrtc2Request(address, dPort, r.URL.Path, "HEAD", r.Header, nil)
	if err != nil {
		return err
	}
	ct := headRes.Header["Content-Type"]
	regl := regexp.MustCompile(`^application`)
	isDownload := regl.Match([]byte(ct))
	if !isDownload {
		return fmt.Errorf("id not download link")
	}
	rw.WriteHeader(205)
	s.emitDownloadFile(dPort, fmt.Sprintf("http://%s%s", r.Host, r.URL.Path))
	return nil
}

func (s *Service) emitDownloadFile(
	dPort string,
	url string,
) {
	{
		wsMap, found := s.dPortWsMap.Load(dPort)
		if found {
			for _, ws := range wsMap.(map[string]*websocket.Conn) {
				err := ws.WriteJSON(&DwebWsMessage{dPort, url, "pageDownload"})
				if err != nil {
					s.logger.Debugf("WriteJSON %s %s failed %s", dPort, url, err.Error())
				}
			}
		}
	}
	{
		wsMap, found := s.dPortWsMap.Load("*")
		if found {
			for _, ws := range wsMap.(map[string]*websocket.Conn) {
				err := ws.WriteJSON(&DwebWsMessage{dPort, url, "pageDownload"})
				if err != nil {
					s.logger.Debugf("WriteJSON* %s %s failed %s", dPort, url, err.Error())
				}
			}
		}
	}
}

func (s *Service) bnrtc2Request(
	address string,
	dPort string,
	url string,
	method string,
	reqHeader map[string][]string,
	postForm map[string][]string,
) (*HttpProxyResponse, error) {
	req := s.reqRespManager.GetReq()
	httpReq := &HttpProxyRequest{req.Id(), url, method, reqHeader, postForm}
	reqBytes, err := json.Marshal(httpReq)
	if err != nil {
		s.reqRespManager.CancelReq(req.Id())
		return &HttpProxyResponse{StatusCode: 404}, err
	}

	err = s.BnrtcClient.Send("", address, dPort, "", reqBytes)
	s.logger.Debugf("send bnrtc client message, %s, %s, %v, %v", address, dPort, req, err)
	if err != nil {
		s.reqRespManager.CancelReq(req.Id())
		return &HttpProxyResponse{StatusCode: 404}, err
	}

	resp := req.WaitResult()
	if resp == nil || !resp.IsSuccess() {
		s.logger.Debugf("bnrtc client on dweb message, address: %s, dPort: %s, err: %v", address, dPort, resp.Error())
		return &HttpProxyResponse{StatusCode: 504}, fmt.Errorf("request bnrtc time out")
	} else {
		httpResp := resp.Data().(*HttpProxyResponse)
		s.logger.Debugf("bnrtc client on dweb message, address: %s, dPort: %s, dataLength: %v", address, dPort, len(httpResp.Data))
		return httpResp, nil
	}
}

func (s *Service) proxyBnrtc2Request(
	address string,
	dPort string,
	rw http.ResponseWriter,
	r *http.Request,
) {
	downloadErr := s.requestDownload(address, dPort, rw, r)
	if downloadErr == nil {
		return
	}
	if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
		_ = r.ParseForm()
	}
	proxyRes, err := s.bnrtc2Request(address, dPort, r.URL.Path, r.Method, r.Header, r.PostForm)
	if proxyRes.StatusCode == 404 {
		s.handlerNotFountPage(rw, r)
		return
	}
	if err != nil && proxyRes.StatusCode == 0 {
		s.logger.Errorf("send error bnrtc client, error: %s", err.Error())
		return
	}
	if proxyRes.StatusCode >= 200 && proxyRes.StatusCode < 300 {
		if proxyRes.Header != nil {
			for key, value := range proxyRes.Header {
				rw.Header().Set(key, value)
			}
		}
		rw.WriteHeader(int(proxyRes.StatusCode))
		_, err = rw.Write(proxyRes.Data)
		if err != nil {
			s.logger.Debugf("write response error, error: %s", err.Error())
		}
	} else {
		rw.WriteHeader(int(proxyRes.StatusCode))
	}
}

func (s *Service) handlerNotFountPage(
	rw http.ResponseWriter,
	r *http.Request,
) {
	if s.notFoundPageByte != nil {
		accept := r.Header.Get("accept")
		regl := regexp.MustCompile(`[html][xml]`)
		isHtml := regl.Match([]byte(accept))
		if isHtml {
			rw.Header().Set("content-type", "text/html")
			_, err := rw.Write(s.notFoundPageByte)
			if err != nil {
				s.logger.Debugf("write response error, error: %s", err.Error())
			}
			return
		}
	}
	rw.WriteHeader(404)
}

func decodeDwebResponce(data []byte, res *HttpProxyResponse) error {
	dataRead := bytes.NewBuffer(data)
	err := binary.Read(dataRead, binary.BigEndian, &res.ReqId)
	if err != nil {
		return err
	}
	err = binary.Read(dataRead, binary.BigEndian, &res.StatusCode)
	if err != nil {
		return err
	}
	var headerLength uint16
	err = binary.Read(dataRead, binary.BigEndian, &headerLength)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data[8:8+headerLength], &res.Header)
	if err != nil {
		return err
	}
	res.Data = data[8+headerLength:]
	return nil
}

func encodeDwebResponce(res *HttpProxyResponse) ([]byte, error) {
	buf := make([]byte, 6)
	binary.BigEndian.PutUint32(buf, res.ReqId)
	binary.BigEndian.PutUint16(buf[4:], res.StatusCode)
	headerBuf, err := json.Marshal(res.Header)
	if err != nil {
		return nil, err
	}
	headerLen := len(headerBuf)
	dataWrite := bytes.NewBuffer(buf)
	err = binary.Write(dataWrite, binary.BigEndian, uint16(headerLen))
	if err != nil {
		return nil, err
	}
	dataWrite.Write(headerBuf)
	dataWrite.Write(res.Data)
	return dataWrite.Bytes(), nil
}
