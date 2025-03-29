package niu

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Authenticator 需要处理以下内容：
// 1. 验证请求是否合法有效
// 2. 验证请求是否被篡改
// 3. 验证请求是否被重复使用
// 4. 跨域问题
type Authenticator interface {
	AuthenticateMiddleware(c *gin.Context)
}

type ReplayHandler interface {
	IsReplayRequest(requestId string, timestamp int64) bool
	MarkRequestHandled(id string) error
}

type DefaultAuthenticator struct {
	replayHandler      ReplayHandler
	signer             Signer
	cryptor            Cryptor
	cryptPaths         []string          // 如果包含*号，表示所有请求都是加密请求
	cryptExcludePaths  []string          // 指定哪些请求不加密，优先级高于cryptPaths
	allowMethods       []string          // 如果为空，表示允许所有请求
	decryptContentType map[string]string // 解密后内容的content-type，默认为 application/json

	bufferPool sync.Pool
}

func NewAuthenticator() *DefaultAuthenticator {
	d := &DefaultAuthenticator{}
	d.bufferPool = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 0, 1024))
		},
	}

	return d
}

func (d *DefaultAuthenticator) getBuffer() *bytes.Buffer {
	buf := d.bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func (d *DefaultAuthenticator) isMethodAllowed(c *gin.Context) bool {
	if len(d.allowMethods) == 0 {
		return true
	}
	for _, method := range d.allowMethods {
		if strings.EqualFold(method, c.Request.Method) {
			return true
		}
	}
	return false
}
func (d *DefaultAuthenticator) isPathEncrypted(path string) bool {
	for _, p := range d.cryptExcludePaths {
		if strings.EqualFold(p, path) {
			return false
		}
	}
	for _, p := range d.cryptPaths {
		if strings.Contains(p, "*") {
			return true
		}
		if strings.EqualFold(p, path) {
			return true
		}
	}
	return false
}
func (d *DefaultAuthenticator) getDecryptContentType(path string) string {
	if len(d.decryptContentType) == 0 {
		return ContentTypeJson
	}
	for p, contentType := range d.decryptContentType {
		if strings.EqualFold(p, path) {
			return contentType
		}
	}
	return ContentTypeJson
}

func (d *DefaultAuthenticator) AuthenticateMiddleware(c *gin.Context) {
	if !d.isMethodAllowed(c) {
		c.AbortWithStatus(405)
		return
	}

	nonce := strings.TrimSpace(c.GetHeader("X-Nonce"))
	timestampStr := strings.TrimSpace(c.GetHeader("X-Timestamp"))
	platform := strings.TrimSpace(c.GetHeader("X-Platform"))
	signature := c.GetHeader("X-Signature")
	if len(nonce) == 0 || len(timestampStr) == 0 || len(signature) == 0 || !IsPlatformStringValid(platform) {
		c.AbortWithStatus(400)
		return
	}

	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		c.AbortWithError(400, errors.New("timestamp is invalid"))
		return
	}

	// 1. 验证请求是否是重放请求
	if d.replayHandler.IsReplayRequest(nonce, timestamp) {
		c.AbortWithError(400, errors.New("repeat request"))
		return
	}
	err = d.replayHandler.MarkRequestHandled(nonce)
	if err != nil {
		c.AbortWithStatus(500)
		return
	}

	// 2. 验证请求是否被篡改
	reqBody := make([]byte, 0, 0)
	if c.Request.Body != nil {
		reqBody, err = io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatus(500)
			return
		}
	}

	// 3. 先验证签名是否正确，在根据需要解密请求体
	signdata := d.stringfySignData(map[string]string{
		"nonce":     nonce,
		"timestamp": timestampStr,
		"platform":  platform,
		"method":    c.Request.Method,
		"path":      c.Request.URL.Path,
		"query":     c.Request.URL.RawQuery,
		"body":      string(reqBody),
	})
	if !d.signer.Verify(signdata, []byte(signature)) {
		c.AbortWithError(400, errors.New("invalid signature"))
		return
	}

	// 4. 修改并重置请求体: 需验证请求是否加密，如果加密，则解密
	if len(reqBody) > 0 {
		if d.isPathEncrypted(c.Request.URL.Path) {
			contentType := c.GetHeader("Content-Type")
			if !strings.EqualFold(contentType, ContentTypeEncrypted) {
				c.AbortWithStatus(400)
				return
			}
			// 解密
			reqBody, err = d.cryptor.Decrypt(reqBody)
			if err != nil {
				c.AbortWithError(400, errors.New("decrypt fail"))
				return
			}

			c.Request.Header.Set("Content-Type", d.getDecryptContentType(c.Request.URL.Path))
		}

		buf := d.getBuffer()
		buf.Write(reqBody)
		c.Request.Body = io.NopCloser(buf)
		c.Request.ContentLength = int64(len(reqBody))
	}

	// 3. 代理响应写入器
	bodyWriter := &bodyWriter{ResponseWriter: c.Writer, buf: bytes.NewBuffer(make([]byte, 0, 1024))}
	c.Writer = bodyWriter

	// 4. 处理业务逻辑
	c.Next()

	// 5. 如果需要加密，则加密响应内容
	responseBody := bodyWriter.buf.Bytes()
	respContentType := c.Writer.Header().Get("Content-Type")
	if d.isPathEncrypted(c.Request.URL.Path) {
		responseBody, err = d.cryptor.Encrypt(responseBody)
		if err != nil {
			c.AbortWithError(500, errors.New("encrypt fail"))
			return
		}
		respContentType = ContentTypeEncrypted
	}

	// 6. 生成响应签名
	respTimestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	respNonce := NewUUIDWithoutDash()
	respSignData := d.stringfySignData(map[string]string{
		"nonce":     respNonce,
		"platform":  platform,
		"timestamp": respTimestamp,
		"method":    c.Request.Method,
		"path":      c.Request.RequestURI,
		"query":     c.Request.URL.RawQuery,
		"body":      string(responseBody),
	})
	respSignature, err := d.signer.Sign(respSignData)
	if err != nil {
		c.AbortWithError(500, errors.New("sign fail"))
	}
	c.Header("X-Signature", respTimestamp)
	c.Header("X-Nonce", respNonce)
	c.Header("X-Timestamp", string(respSignature))
	// browser need this, or it cannot read these headers
	c.Header("Access-Control-Expose-Headers", "X-Timestamp,X-Nonce,X-Signature")
	c.Header(HeaderContentType, respContentType)
	c.Writer.Write(responseBody)
}

// 用于生成待签名的内容
func (d *DefaultAuthenticator) stringfySignData(params map[string]string) []byte {
	// 对参数名进行排序
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 拼接参数
	b := strings.Builder{}
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("%s=%s\n", k, params[k]))
	}
	return []byte(b.String())
}

// 自定义响应写入器
type bodyWriter struct {
	gin.ResponseWriter
	buf *bytes.Buffer
}

func (w bodyWriter) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

// Golang 生成 HMAC 签名
func GenerateHMAC(secret, data string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}
