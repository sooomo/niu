package protocols

import (
	"encoding/json"
	"errors"
	"niu/crypto"
	"time"
)

type JsonProtocol struct {
	Timestamp int32
	SeqNumber byte
	Signer    crypto.Signer
	Cryptor   crypto.Cryptor
}

func (m *JsonProtocol) EncodeReq(msgType byte, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	m.Timestamp = int32(time.Now().Sub(protocolStartTime).Seconds())
	m.SeqNumber++

	out := []byte{byte(msgType)}
	out = append(out, byte(m.Timestamp>>24), byte(m.Timestamp>>16), byte(m.Timestamp>>8), byte(m.Timestamp))
	if m.Cryptor != nil {
		body, err = m.Cryptor.Encrypt(body)
		if err != nil {
			return nil, err
		}
	}

	if m.Signer != nil {
		dataToSign := append(out, body...)
		signature, err := m.Signer.Sign(dataToSign)
		if err != nil {
			return nil, err
		}

		out = append(out, signature...)
	}

	return append(out, body...), nil
}

func (m *JsonProtocol) Decode(data []byte, payload any) (msgType, code byte, err error) {
	if len(data) < 7 {
		return 0, 0, errors.New("Bad Data Format")
	}
	msgType = data[0]
	ts := int64(data[1])<<24 | int64(data[2])<<16 | int64(data[3])<<8 | int64(data[4])
	m.Timestamp = int32(protocolStartTime.Add(time.Duration(ts) * time.Second).Unix())
	m.SeqNumber = data[5]
	code = data[6]
	body := data[7:]

	if m.Signer != nil {
		end := 7 + m.Signer.Len()
		if len(data) < end {
			return 0, 0, errors.New("Bad Data Format: No Sign")
		}
		signBytes := data[7:end]
		body = data[end:]
		dataToVerify := data[:7]
		dataToVerify = append(dataToVerify, body...)
		if !m.Signer.Verify(dataToVerify, signBytes) {
			return 0, 0, errors.New("Sign verify fail")
		}
	}

	if m.Cryptor != nil {
		body, err = m.Cryptor.Decrypt(body)
		if err != nil {
			return 0, 0, err
		}
	}

	if err = json.Unmarshal(body, payload); err != nil {
		return 0, 0, err
	}
	return msgType, code, nil
}
