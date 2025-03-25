package niu

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"

	mrand "math/rand"
)

// 生成随机数
func Random(min, max int) int {
	// mrand.New(mrand.NewSource(time.Now().UnixNano()))
	return mrand.Intn(max-min) + min
}

// Md5
func HashMd5(str string) string {
	data := []byte(str)
	hash := md5.Sum(data)
	md5Str := hex.EncodeToString(hash[:])
	return md5Str
}

// Sha256
func HashSha256(str string) string {
	data := []byte(str)
	hash := sha256.Sum256(data)
	sha256Str := hex.EncodeToString(hash[:])
	return sha256Str
}

// 生成强密码
func SecureBytes(keyLenInBytes int) ([]byte, error) {
	if keyLenInBytes <= 0 {
		return nil, errors.New("keyLenInBytes must be >= 0")
	}

	bytes := make([]byte, keyLenInBytes)
	// ReadFull从rand.Reader精确地读取len(b)字节数据填充进b
	// rand.Reader是一个全局、共享的密码用强随机数生成器
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return nil, err
	}

	return bytes, nil
}

// 编码为 base64 字符串
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// 解码base64
func Base64Decode(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(data)
}

// 编码为 base64 字符串
func Base64URLEncode(data []byte) string {
	return base64.URLEncoding.EncodeToString(data)
}

// 解码base64
func Base64URLDecode(data string) ([]byte, error) {
	return base64.URLEncoding.DecodeString(data)
}
