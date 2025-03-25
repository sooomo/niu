package niu

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"

	"golang.org/x/crypto/curve25519"
)

type Cryptor interface {
	Encrypt(rawData []byte) ([]byte, error)
	EncryptToString(rawData []byte) (string, error)
	Decrypt(ciphertext []byte) ([]byte, error)
	DecryptFromString(ciphertext string) ([]byte, error)
}

type RsaCryptor struct {
	PublicKey  []byte // PEM format: 即有-----BEGIN XXX KEY-----...-----END XXX KEY-----
	PrivateKey []byte // PEM format: 即有-----BEGIN XXX KEY-----...-----END XXX KEY-----
}

func (r *RsaCryptor) Encrypt(rawData []byte) ([]byte, error) {
	block, _ := pem.Decode(r.PublicKey)
	if block == nil {
		return nil, errors.New("public key error")
	}
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub := pubInterface.(*rsa.PublicKey)
	return rsa.EncryptPKCS1v15(rand.Reader, pub, rawData)
}

func (r *RsaCryptor) EncryptToString(rawData []byte) (string, error) {
	cipherBytes, err := r.Encrypt(rawData)
	if err != nil {
		return "", err
	}

	return Base64Encode(cipherBytes), nil
}

func (r *RsaCryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	block, _ := pem.Decode(r.PrivateKey)
	if block == nil {
		return nil, errors.New("private key error")
	}
	parseResult, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("parse error")
	}
	priv := parseResult.(*rsa.PrivateKey)
	return rsa.DecryptPKCS1v15(rand.Reader, priv, ciphertext)
}

func (r *RsaCryptor) DecryptFromString(ciphertext string) ([]byte, error) {
	data, err := Base64Decode(ciphertext)
	if err != nil {
		return nil, err
	}

	return r.Decrypt(data)
}

func NewRsaCryptor(publicKey, privateKey []byte) *RsaCryptor {
	return &RsaCryptor{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}
}

type Ed25519Cryptor struct {
	SharedKey []byte // 与服务端协商的密钥
}

// 将密钥转换为 base64 格式字符串
func (e *Ed25519Cryptor) SharedKeyString() string {
	return Base64Encode(e.SharedKey)
}

// 对指定输入加密，结果为: nonce + cipherData + tag
func (e *Ed25519Cryptor) Encrypt(rawData []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.SharedKey)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce, err := SecureBytes(aesgcm.NonceSize())
	if err != nil {
		return nil, err
	}

	cipherData := aesgcm.Seal(nonce, nonce, rawData, nil)
	// cipherData = append(cipherData, nonce...)
	return cipherData, nil
}

func (e *Ed25519Cryptor) EncryptToString(rawData []byte) (string, error) {
	cipherBytes, err := e.Encrypt(rawData)
	if err != nil {
		return "", err
	}

	return Base64Encode(cipherBytes), nil
}

// 对指定输入解密，输入为: nonce + cipherData + tag
func (e *Ed25519Cryptor) Decrypt(rawData []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.SharedKey)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := rawData[:aesgcm.NonceSize()]
	cipherData := rawData[aesgcm.NonceSize():]

	outData, err := aesgcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return nil, err
	}

	return outData, nil
}

func (e *Ed25519Cryptor) DecryptFromString(ciphertext string) ([]byte, error) {
	data, err := Base64Decode(ciphertext)
	if err != nil {
		return nil, err
	}

	return e.Decrypt(data)
}

// 初始化一个加密器
func NewEd25519Cryptor(sharedKey string) (*Ed25519Cryptor, error) {
	sharedKeyBytes, err := Base64Decode(sharedKey)
	if err != nil {
		return nil, err
	}

	return &Ed25519Cryptor{SharedKey: sharedKeyBytes}, nil
}

// 通过与对方公钥协商，获得共享密钥
func NewEd25519CryptorByNegotiate(remotePublicKey, selfPrivateKey []byte) (*Ed25519Cryptor, error) {
	sharedKey, err := curve25519.X25519(selfPrivateKey, remotePublicKey)
	if err != nil {
		return nil, err
	}
	return &Ed25519Cryptor{SharedKey: sharedKey}, nil
}

// 生成 Ed25519 密钥对
func NewEd25519CryptorKeyPair() (pubKey, priKey []byte, err error) {
	private, err := SecureBytes(32)
	if err != nil {
		return nil, nil, err
	}

	private[0] &= 248
	private[31] &= 127
	private[31] |= 64

	public, err := curve25519.X25519(private[:], curve25519.Basepoint)
	if err != nil {
		return nil, nil, err
	}

	return public, private, nil
}
