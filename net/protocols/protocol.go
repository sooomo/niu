package protocols

import (
	"time"
)

type MessageProtocol interface {
	EncodeReq(msgType byte, payload any) ([]byte, error)
	EncodeResp(msgType, code byte, payload any) ([]byte, error)
	DecodeReq(data []byte, payload any) (msgType byte, err error)
	DecodeResp(data []byte, payload any) (msgType, code byte, err error)
}

var protocolStartTime = time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)
