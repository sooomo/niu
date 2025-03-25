package net

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sooomo/niu/crypto"
	"github.com/sooomo/niu/id"

	"github.com/gin-gonic/gin"
)

type ReplyDto[TCode any, TData any] struct {
	Code TCode  `json:"code"`
	Msg  string `json:"msg"`
	Data TData  `json:"data"`
}

var (
	HeaderSignTimestamp string
	HeaderSignNonce     string
	HeaderSignSignature string
)

func InitSignHeaders(bizType string) {
	HeaderSignTimestamp = fmt.Sprintf("x-%s-timestamp", bizType)
	HeaderSignNonce = fmt.Sprintf("x-%s-nonce", bizType)
	HeaderSignSignature = fmt.Sprintf("x-%s-signature", bizType)
}

type ServerSignConfig struct {
	PriKey string
	PubKey string
	Signer *crypto.Ed25519SignKey
}

// 自定义响应写入器
type bodyWriter struct {
	gin.ResponseWriter
	body *strings.Builder
}

func (w bodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// 用于生成待签名的内容
func DefaultSignRule(params map[string]string) []byte {
	// 对参数名进行排序
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 拼接参数
	b := strings.Builder{}
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("%s=%s", k, params[k]))
	}
	return []byte(b.String())
}

// 防止重放攻击的中间件
func ReplayInterceptMiddleware(canNext func(reqId string) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		reqNonce := c.GetHeader(HeaderSignNonce)
		if !canNext(reqNonce) {
			c.AbortWithError(http.StatusForbidden, errors.New("no replay"))
			return
		}
		c.Next()
	}
}

// 签名及验证的中间件
func SignatureMiddleware(signerGetter func(ctx *gin.Context) crypto.Signer, signRule func(mp map[string]string) []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		if signRule == nil {
			signRule = DefaultSignRule
		}
		signer := signerGetter(c)
		if signer == nil {
			c.AbortWithError(http.StatusInternalServerError, errors.New("signer get fail"))
			return
		}

		reqBody := []byte{}
		if c.Request.Body != nil {
			reqBody, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Read body fail",
				})
				return
			}
			// 修改并重置请求体
			c.Request.Body = io.NopCloser(bytes.NewBuffer(reqBody))
			c.Request.ContentLength = int64(len(reqBody))
		}

		// 验证签名
		reqSign := c.GetHeader(HeaderSignSignature)
		reqTimestamp := c.GetHeader(HeaderSignTimestamp)
		reqNonce := c.GetHeader(HeaderSignNonce)
		reqSignData := signRule(map[string]string{
			"method":    c.Request.Method,
			"path":      c.Request.RequestURI,
			"query":     c.Request.URL.RawQuery,
			"timestamp": reqTimestamp,
			"body":      string(reqBody),
			"nonce":     reqNonce,
		})
		if !signer.Verify(reqSignData, []byte(reqSign)) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Invalid signature",
			})
			return
		}

		// 记录原始响应写入器
		w := c.Writer
		// 创建自定义响应写入器
		bodyWriter := &bodyWriter{ResponseWriter: w, body: &strings.Builder{}}
		c.Writer = bodyWriter

		// 继续处理请求
		c.Next()

		// 获取响应内容
		responseBody := bodyWriter.body.String()

		// 生成响应签名
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		nonce := id.NewUUIDWithoutDash()
		respSignData := signRule(map[string]string{
			"method":    c.Request.Method,
			"path":      c.Request.RequestURI,
			"query":     c.Request.URL.RawQuery,
			"timestamp": timestamp,
			"body":      responseBody,
			"nonce":     nonce,
		})
		respSign, err := signer.Sign(respSignData)
		if err != nil {
			c.String(http.StatusInternalServerError, "%s", "sign resp fail")
		}
		c.Header(HeaderSignTimestamp, timestamp)
		c.Header(HeaderSignNonce, nonce)
		c.Header(HeaderSignSignature, string(respSign))
		// browser need this, or it cannot read these headers
		c.Header("Access-Control-Expose-Headers", fmt.Sprintf("%v,%v,%v", HeaderSignTimestamp, HeaderSignNonce, HeaderSignSignature))
	}
}

// 加解密内容的中间件
func CryptoMiddleware(cryptorGetter func(ctx *gin.Context) crypto.Cryptor) gin.HandlerFunc {
	return func(c *gin.Context) {
		cryptor := cryptorGetter(c)
		if cryptor == nil {
			c.AbortWithError(http.StatusInternalServerError, errors.New("cryptor get fail"))
			return
		}
		if c.Request.Body != nil {
			reqBody, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, errors.New("body read fail"))
				return
			}
			// 解密
			reqBody, err = cryptor.Decrypt(reqBody)
			if err != nil {
				c.AbortWithError(http.StatusBadRequest, errors.New("decrypt fail"))
				return
			}
			// 修改并重置请求体
			c.Request.Body = io.NopCloser(bytes.NewBuffer(reqBody))
			c.Request.ContentLength = int64(len(reqBody))
		}

		// 记录原始响应写入器
		w := c.Writer
		// 创建自定义响应写入器
		bodyWriter := &bodyWriter{ResponseWriter: w, body: &strings.Builder{}}
		c.Writer = bodyWriter

		// 继续处理请求
		c.Next()

		// 获取响应内容
		responseBody := bodyWriter.body.String()

		// 加密
		out, err := cryptor.Encrypt([]byte(responseBody))
		if err != nil {
			c.String(http.StatusInternalServerError, "%s", "sign resp fail")
			return
		}
		c.Header(HeaderContentType, ContentTypeSec)
		c.Writer.Write(out)
	}
}
