package protocols

import (
	"encoding/json"

	"github.com/shamaton/msgpack/v2"
)

type PayloadMarshaler interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

type MsgPackMarshaler struct{}

var msgpackMarshaler = &MsgPackMarshaler{}

func (m *MsgPackMarshaler) Marshal(v any) ([]byte, error) {
	return msgpack.Marshal(v)
}
func (m *MsgPackMarshaler) Unmarshal(data []byte, v any) error {
	return msgpack.Unmarshal(data, v)
}

type JsonMarshaler struct{}

var jsonMarshaler = &JsonMarshaler{}

func (m *JsonMarshaler) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
func (m *JsonMarshaler) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
