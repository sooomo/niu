package niu

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

type Signer interface {
	Sign(rawData []byte) ([]byte, error)
	Verify(utf8Bytes []byte, signature []byte) bool
	SignatureLen() int
}

type HmacSigner struct {
	secretKey []byte
}

func NewHmacSigner(secret []byte) *HmacSigner {
	return &HmacSigner{secretKey: secret}
}

func (h *HmacSigner) SignatureLen() int { return 32 }

// Golang 生成 HMAC 签名
func (h *HmacSigner) Sign(data []byte) ([]byte, error) {
	ha := hmac.New(sha256.New, h.secretKey)
	ha.Write(data)
	return ha.Sum(nil), nil
}

func (h *HmacSigner) Verify(dataToSign []byte, signature []byte) bool {
	ha := hmac.New(sha256.New, h.secretKey)
	ha.Write(dataToSign)
	sign := ha.Sum(nil)
	return hmac.Equal(sign, signature)
}

type Ed25519Signer struct {
	RemotePublicKey ed25519.PublicKey  // 远端的公钥，用于验证远程发过来的数据的签名
	SelfPrivateKey  ed25519.PrivateKey // 本地的私钥，用于对发往服务器的数据进行签名
}

func (e *Ed25519Signer) SignatureLen() int { return 64 }

func (e *Ed25519Signer) RemotePublicKeyString() string {
	return Base64Encode(e.RemotePublicKey)
}

func (e *Ed25519Signer) SelfPrivateKeyString() string {
	return Base64Encode(e.SelfPrivateKey)
}

// 对指定输入进行签名
func (e *Ed25519Signer) Sign(rawData []byte) ([]byte, error) {
	return ed25519.Sign(e.SelfPrivateKey, rawData), nil
}

// 验证指定输入的签名
func (e *Ed25519Signer) Verify(utf8Bytes []byte, signature []byte) bool {
	return ed25519.Verify(e.RemotePublicKey, utf8Bytes, signature)
}

// 初始化一个签名器
func NewEd25519SignerFromString(remotePublicKey, selfPrivateKey string) (*Ed25519Signer, error) {
	remotePublicKeyBytes, err := Base64Decode(remotePublicKey)
	if err != nil {
		return nil, err
	}
	selfPrivateKeyBytes, err := Base64Decode(selfPrivateKey)
	if err != nil {
		return nil, err
	}

	signer := &Ed25519Signer{RemotePublicKey: remotePublicKeyBytes, SelfPrivateKey: selfPrivateKeyBytes}
	return signer, nil
}

// 初始化一个Ed25519 密钥对
func NewEd25519SignerKeyPair() (pubKey, priKey []byte, err error) {
	return ed25519.GenerateKey(nil)
}

// 初始化一个签名器
func NewEd25519Signer(remotePublicKey []byte, selfPrivateKey []byte) *Ed25519Signer {
	signer := &Ed25519Signer{RemotePublicKey: remotePublicKey, SelfPrivateKey: selfPrivateKey}
	return signer
}

// Pri, Pub需要公开出去以供配置文件加载程序使用
type Ed25519SignKey struct {
	Pri    string
	Pub    string
	priKey ed25519.PrivateKey
}

func NewEd25519SignKey(pri, pub string) (*Ed25519SignKey, error) {
	tmp := &Ed25519SignKey{Pri: pri, Pub: pub}
	k, err := tmp.getPrivateKey()
	if err != nil {
		return nil, err
	}
	tmp.priKey = k
	return tmp, nil
}

func (sk *Ed25519SignKey) getPrivateKey() (ed25519.PrivateKey, error) {
	priBytes, err := base64.StdEncoding.DecodeString(sk.Pri)
	if err != nil {
		return nil, err
	}
	pubBytes, err := base64.StdEncoding.DecodeString(sk.Pub)
	if err != nil {
		return nil, err
	}
	key := append(priBytes, pubBytes...)
	return ed25519.PrivateKey(key), nil
}

func (sk *Ed25519SignKey) Sign(message []byte) []byte {
	return ed25519.Sign(sk.priKey, message)
}

func (sk *Ed25519SignKey) SignString(message string) string {
	data := ed25519.Sign(sk.priKey, []byte(message))
	return base64.StdEncoding.EncodeToString(data)
}
