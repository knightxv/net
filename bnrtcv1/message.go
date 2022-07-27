package bnrtcv1

import (
	"encoding/json"
	"net/url"
)

type MessageType int8

const (
	MessageTypeCtrl MessageType = iota
	MessageTypeData
	MessageTypeClose
)

type CtrlMessageType int8

const (
	CtrlMessageSetNatName CtrlMessageType = iota
	CtrlMessageNatSend
	CtrlMessageNatData
)

type MessageHeader struct {
	MsgId   uint32      // `json:"reqId"`
	MsgType MessageType //`json:"msgType"`
	IsReq   bool        //`json:"isReq"`
}

type CtrlMessageHeader struct {
	MessageHeader
	CtrlMessageType //`json:"type"`
}

type CtrlNatName struct {
	CtrlMessageHeader
	Name string // `json:"name"`
}

type CtrlNatRequest struct {
	CtrlMessageHeader
	URL      url.URL    // `json:"url"`
	Method   string     // `json:"method"`
	PostForm url.Values // `json:"postForm"`
}

type CtrlNatResponse struct {
	CtrlMessageHeader
	Content    []byte // `json:"content"`
	StatusCode int    // `json:"statusCode"`
}
type DataMessage struct {
	MessageHeader
	Data []byte // `json:"data"`
}

func NewCtrlNatNameMessage(name string) *CtrlNatName {
	return &CtrlNatName{
		CtrlMessageHeader: CtrlMessageHeader{
			MessageHeader: MessageHeader{
				MsgType: MessageTypeCtrl,
				IsReq:   true,
			},
			CtrlMessageType: CtrlMessageSetNatName,
		},
		Name: name,
	}
}

func NewCtrlNatRequest(url url.URL, method string, posForm url.Values) *CtrlNatRequest {
	return &CtrlNatRequest{
		CtrlMessageHeader: CtrlMessageHeader{
			MessageHeader: MessageHeader{
				MsgType: MessageTypeCtrl,
				IsReq:   true,
			},
			CtrlMessageType: CtrlMessageNatSend,
		},
		URL:      url,
		Method:   method,
		PostForm: posForm,
	}
}

func NewCtrlNatResponse(msgId uint32, content []byte, statusCode int) *CtrlNatResponse {
	return &CtrlNatResponse{
		CtrlMessageHeader: CtrlMessageHeader{
			MessageHeader: MessageHeader{
				MsgType: MessageTypeCtrl,
				IsReq:   false,
				MsgId:   msgId,
			},
			CtrlMessageType: CtrlMessageNatSend,
		},
		Content:    content,
		StatusCode: statusCode,
	}
}

func NewDataMessage(data []byte) *DataMessage {
	return &DataMessage{
		MessageHeader: MessageHeader{
			MsgType: MessageTypeData,
			IsReq:   true,
		},
		Data: data,
	}
}

func (req *MessageHeader) fromBytes(reqBytes []byte) error {
	return json.Unmarshal(reqBytes, req)
}
func (req *MessageHeader) toBytes() ([]byte, error) {
	return json.Marshal(req)
}
func (req *CtrlMessageHeader) fromBytes(reqBytes []byte) error {
	return json.Unmarshal(reqBytes, req)
}
func (req *CtrlMessageHeader) toBytes() ([]byte, error) {
	return json.Marshal(req)
}
func (req *CtrlNatName) fromBytes(reqBytes []byte) error {
	return json.Unmarshal(reqBytes, req)
}
func (req *CtrlNatName) toBytes() ([]byte, error) {
	return json.Marshal(req)
}
func (req *CtrlNatRequest) fromBytes(reqBytes []byte) error {
	return json.Unmarshal(reqBytes, req)
}
func (req *CtrlNatRequest) toBytes() ([]byte, error) {
	return json.Marshal(req)
}
func (res *CtrlNatResponse) fromBytes(resBytes []byte) error {
	return json.Unmarshal(resBytes, res)
}
func (res *CtrlNatResponse) toBytes() ([]byte, error) {
	return json.Marshal(res)
}
func (res *DataMessage) fromBytes(resBytes []byte) error {
	return json.Unmarshal(resBytes, res)
}
func (res *DataMessage) toBytes() ([]byte, error) {
	return json.Marshal(res)
}
