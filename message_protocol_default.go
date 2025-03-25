package niu

import (
	"errors"
	"time"
)

type DefaultMessageProtocol struct {
	timestamp time.Time
	seqNumber byte
	signer    Signer
	cryptor   Cryptor
	marshaler PayloadMarshaler
}

func NewMsgPackProtocol(signer Signer, cryptor Cryptor) *DefaultMessageProtocol {
	return &DefaultMessageProtocol{
		signer:    signer,
		cryptor:   cryptor,
		marshaler: msgpackMarshaler,
	}
}

func NewJsonProtocol(signer Signer, cryptor Cryptor) *DefaultMessageProtocol {
	return &DefaultMessageProtocol{
		signer:    signer,
		cryptor:   cryptor,
		marshaler: jsonMarshaler,
	}
}

func (m *DefaultMessageProtocol) GetTimestamp() time.Time { return m.timestamp }

func (m *DefaultMessageProtocol) GetSeqNumber() int { return int(m.seqNumber) }

func (m *DefaultMessageProtocol) GetMeta(data []byte) (msgType byte, timestamp time.Time, seqNum byte, err error) {
	if len(data) < 6 {
		return 0, time.Now(), 0, errors.New("Bad Data Format")
	}
	ts := int64(data[1])<<24 | int64(data[2])<<16 | int64(data[3])<<8 | int64(data[4])
	timestamp = protocolStartTime.Add(time.Duration(ts) * time.Second)
	return data[0], timestamp, data[5], nil
}

func (m *DefaultMessageProtocol) EncodeReq(msgType byte, payload any) ([]byte, error) {
	body, err := m.marshaler.Marshal(payload)
	if err != nil {
		return nil, err
	}
	m.timestamp = time.Now()
	timestamp := int32(m.timestamp.Sub(protocolStartTime).Seconds())
	m.seqNumber++

	out := []byte{byte(msgType)}
	out = append(out, byte(timestamp>>24), byte(timestamp>>16), byte(timestamp>>8), byte(timestamp))
	out = append(out, m.seqNumber)

	if m.cryptor != nil {
		body, err = m.cryptor.Encrypt(body)
		if err != nil {
			return nil, err
		}
	}

	if m.signer != nil {
		dataToSign := append(out, body...)
		signature, err := m.signer.Sign(dataToSign)
		if err != nil {
			return nil, err
		}

		out = append(out, signature...)
	}

	return append(out, body...), nil
}

func (m *DefaultMessageProtocol) EncodeResp(msgType, code byte, payload any) ([]byte, error) {
	body, err := m.marshaler.Marshal(payload)
	if err != nil {
		return nil, err
	}

	timestamp := int32(m.timestamp.Sub(protocolStartTime).Seconds())
	out := []byte{byte(msgType)}
	out = append(out, byte(timestamp>>24), byte(timestamp>>16), byte(timestamp>>8), byte(timestamp))
	out = append(out, m.seqNumber, code)

	if m.cryptor != nil {
		body, err = m.cryptor.Encrypt(body)
		if err != nil {
			return nil, err
		}
	}

	if m.signer != nil {
		dataToSign := append(out, body...)
		signature, err := m.signer.Sign(dataToSign)
		if err != nil {
			return nil, err
		}

		out = append(out, signature...)
	}

	return append(out, body...), nil
}

func (m *DefaultMessageProtocol) DecodeReq(data []byte, payload any) (msgType byte, err error) {
	if len(data) < 6 {
		return 0, errors.New("Bad Data Format")
	}
	msgType = data[0]
	ts := int64(data[1])<<24 | int64(data[2])<<16 | int64(data[3])<<8 | int64(data[4])
	m.timestamp = protocolStartTime.Add(time.Duration(ts) * time.Second)
	m.seqNumber = data[5]
	body := data[6:]

	if m.signer != nil {
		end := 6 + m.signer.Len()
		if len(data) < end {
			return 0, errors.New("Bad Data Format: No Sign")
		}
		signature := data[6:end]
		body = data[end:]
		dataToVerify := data[:6]
		dataToVerify = append(dataToVerify, body...)
		if !m.signer.Verify(dataToVerify, signature) {
			return 0, errors.New("Sign verify fail")
		}
	}

	if m.cryptor != nil {
		body, err = m.cryptor.Decrypt(body)
		if err != nil {
			return 0, err
		}
	}

	if err = m.marshaler.Unmarshal(body, payload); err != nil {
		return 0, err
	}
	return msgType, nil
}

func (m *DefaultMessageProtocol) DecodeResp(data []byte, payload any) (msgType, code byte, err error) {
	if len(data) < 7 {
		return 0, 0, errors.New("Bad Data Format")
	}
	msgType = data[0]
	ts := int64(data[1])<<24 | int64(data[2])<<16 | int64(data[3])<<8 | int64(data[4])
	m.timestamp = protocolStartTime.Add(time.Duration(ts) * time.Second)
	m.seqNumber = data[5]
	code = data[6]
	body := data[7:]

	if m.signer != nil {
		end := 7 + m.signer.Len()
		if len(data) < end {
			return 0, 0, errors.New("Bad Data Format: No Sign")
		}
		signBytes := data[7:end]
		body = data[end:]
		dataToVerify := data[:7]
		dataToVerify = append(dataToVerify, body...)
		if !m.signer.Verify(dataToVerify, signBytes) {
			return 0, 0, errors.New("Sign verify fail")
		}
	}

	if m.cryptor != nil {
		body, err = m.cryptor.Decrypt(body)
		if err != nil {
			return 0, 0, err
		}
	}

	if err = m.marshaler.Unmarshal(body, payload); err != nil {
		return 0, 0, err
	}
	return msgType, code, nil
}
