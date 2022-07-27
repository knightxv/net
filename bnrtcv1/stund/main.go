package stund

import (
	"fmt"
	"log"
	"net"

	"github.com/pkg/errors"

	"gortc.io/stun"
)

// var (
// 	network = flag.String("net", "udp", "network to listen")
// 	address = flag.String("addr", "0.0.0.0:3478", "address to listen")
// 	profile = flag.Bool("profile", false, "profile")
// )

// Server is RFC 5389 basic server implementation.
//
// Current implementation is UDP only and not utilizes FINGERPRINT mechanism,
// nor ALTERNATE-SERVER, nor credentials mechanisms. It does not support
// backwards compatibility with RFC 3489.
type Server struct {
	// Addr         string
	// LogAllErrors bool
	Conn   net.PacketConn
	Closed bool
}

var (
	software          = stun.NewSoftware("gortc/stund")
	errNotSTUNMessage = errors.New("not stun message")
)

func basicProcess(addr net.Addr, b []byte, req, res *stun.Message) error {
	if !stun.IsMessage(b) {
		return errNotSTUNMessage
	}
	if _, err := req.Write(b); err != nil {
		return errors.Wrap(err, "failed to read message")
	}
	var (
		ip   net.IP
		port int
	)
	switch a := addr.(type) {
	case *net.UDPAddr:
		ip = a.IP
		port = a.Port
	default:
		panic(fmt.Sprintf("unknown addr: %v", addr))
	}
	return res.Build(req,
		stun.BindingSuccess,
		software,
		&stun.XORMappedAddress{
			IP:   ip,
			Port: port,
		},
		stun.Fingerprint,
	)
}

func (s *Server) serveConn(c net.PacketConn, res, req *stun.Message) error {
	if c == nil {
		return nil
	}
	buf := make([]byte, 1024)
	n, addr, err := c.ReadFrom(buf)
	if err != nil {
		log.Printf("ReadFrom: %v", err)
		return nil
	}
	// log.Printf("read %d bytes from %s", n, addr)
	if _, err = req.Write(buf[:n]); err != nil {
		log.Printf("Write: %v", err)
		return err
	}
	if err = basicProcess(addr, buf[:n], req, res); err != nil {
		if err == errNotSTUNMessage {
			return nil
		}
		log.Printf("basicProcess: %v", err)
		return nil
	}
	_, err = c.WriteTo(res.Raw, addr)
	if err != nil {
		log.Printf("WriteTo: %v", err)
	}
	return err
}

// Serve reads packets from connections and responds to BINDING requests.
func (s *Server) Serve() error {
	var (
		res = new(stun.Message)
		req = new(stun.Message)
	)
	for {
		if s.Closed {
			return nil
		}
		if err := s.serveConn(s.Conn, res, req); err != nil {
			log.Printf("serve: %v", err)
			return err
		}
		res.Reset()
		req.Reset()
	}
}

func (s *Server) Close() error {
	defer func() {
		s.Closed = true
	}()
	return s.Conn.Close()
}
