package niu

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

type AuthenticateOption interface {
	apply(*Authenticator)
}

var _ AuthenticateOption = (*optionFunc)(nil)

type optionFunc func(*Authenticator)

func (o optionFunc) apply(c *Authenticator) {
	o(c)
}

// Authenticator 需要处理以下内容：
// 1. 验证请求是否合法有效
// 2. 验证请求是否被篡改
// 3. 验证请求是否被重复使用
// 4. 跨域问题
type Authenticator struct {
	signerResolver     SignerResolver
	cryptorResolver    CryptorResolver
	cryptPaths         []string          // 如果包含*号，表示所有请求都是加密请求
	cryptExcludePaths  []string          // 指定哪些请求不加密，优先级高于cryptPaths
	allowMethods       []string          // 如果为空，表示允许所有请求
	decryptContentType map[string]string // 解密后内容的content-type，默认为 application/json
	authPaths          []string          // 如果包含*号，需要认证的路径
	authExcludePaths   []string          // 认证排除路径，优先级高于authPaths

	bufferPool  sync.Pool
	jwtIssuer   string
	jwtTokenTTL time.Duration
	jwtSecret   []byte

	redisClient *redis.Client
}

type SignerResolver interface {
	Resolve(c *gin.Context) (Signer, error)
}

type HmacSignerResolver struct {
	secret []byte
}

func (a *HmacSignerResolver) Resolve(c *gin.Context) (Signer, error) {
	return &HmacSigner{a.secret}, nil
}

type CryptorResolver interface {
	Resolve(c *gin.Context) (Cryptor, error)
}

func NewHmacSignerResolver(secret []byte) SignerResolver {
	return &HmacSignerResolver{secret: secret}
}

func WithCryptPaths(cryptPaths, excludePaths []string) AuthenticateOption {
	return optionFunc(func(o *Authenticator) {
		o.cryptPaths = cryptPaths
		o.cryptExcludePaths = excludePaths
	})
}
func WithAllowMethods(allowMethods []string) AuthenticateOption {
	return optionFunc(func(o *Authenticator) {
		o.allowMethods = allowMethods
	})
}
func WithDecryptContentType(decryptContentType map[string]string) AuthenticateOption {
	return optionFunc(func(o *Authenticator) {
		o.decryptContentType = decryptContentType
	})
}
func WithAuthPaths(authPaths, excludePaths []string) AuthenticateOption {
	return optionFunc(func(o *Authenticator) {
		o.authPaths = authPaths
		o.authExcludePaths = excludePaths
	})
}
func WithJwt(issuer string, ttl time.Duration, secret []byte) AuthenticateOption {
	return optionFunc(func(o *Authenticator) {
		o.jwtIssuer = issuer
		o.jwtTokenTTL = ttl
		o.jwtSecret = secret
	})
}

func NewAuthenticator(ctx context.Context, redisAddr string, signerResolver SignerResolver, opts ...AuthenticateOption) (*Authenticator, error) {
	d := &Authenticator{
		signerResolver: signerResolver,
		bufferPool: sync.Pool{
			New: func() any {
				return bytes.NewBuffer(make([]byte, 0, 1024))
			},
		},
		redisClient: redis.NewClient(&redis.Options{
			Addr: redisAddr,
		}),
	}
	for _, opt := range opts {
		opt.apply(d)
	}
	if d.signerResolver == nil {
		return nil, errors.New("signer resolver is nil")
	}
	_, err := d.redisClient.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	return d, nil
}

func (d *Authenticator) getBuffer() *bytes.Buffer {
	buf := d.bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func (d *Authenticator) isMethodAllowed(c *gin.Context) bool {
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
func (d *Authenticator) isPathEncrypted(path string) bool {
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
func (d *Authenticator) getDecryptContentType(path string) string {
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
func (d *Authenticator) isPathNeedAuth(path string) bool {
	for _, p := range d.authExcludePaths {
		if strings.EqualFold(p, path) {
			return false
		}
	}
	for _, p := range d.authPaths {
		if strings.Contains(p, "*") {
			return true
		}
		if strings.EqualFold(p, path) {
			return true
		}
	}
	return false
}
func (d *Authenticator) GetPlatform(c *gin.Context) string {
	return strings.TrimSpace(c.GetHeader("X-Platform"))
}

// 用于生成待签名的内容
func (d *Authenticator) stringfySignData(params map[string]string) []byte {
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

func (d *Authenticator) AuthenticateMiddleware(c *gin.Context) {
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

	// 1. 验证请求是否是重放请求
	if !d.checkReplay(c, nonce, timestampStr) {
		return
	}

	// 2. 验证请求是否被篡改
	reqBody := make([]byte, 0, 0)
	if c.Request.Body != nil {
		var err error
		reqBody, err = io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatus(500)
			return
		}
	}

	// 3. 先验证签名是否正确，在根据需要解密请求体
	signer, err := d.signerResolver.Resolve(c)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}
	signdata := d.stringfySignData(map[string]string{
		"nonce":     nonce,
		"timestamp": timestampStr,
		"platform":  platform,
		"method":    c.Request.Method,
		"path":      c.Request.URL.Path,
		"query":     c.Request.URL.RawQuery,
		"body":      string(reqBody),
	})
	if !signer.Verify(signdata, []byte(signature)) {
		c.AbortWithError(400, errors.New("invalid signature"))
		return
	}

	// 4. 解码Token（如果有）
	if !d.verifyToken(c) {
		return
	}

	// 5. 修改并重置请求体: 需验证请求是否加密，如果加密，则解密
	canContinue, cryptor := d.replaceRequestBody(c, reqBody)
	if !canContinue {
		return
	}

	// 6. 代理响应写入器
	respBuf := d.getBuffer()
	bodyWriter := &bodyWriter{ResponseWriter: c.Writer, buf: respBuf}
	c.Writer = bodyWriter

	// 7. 处理业务逻辑
	c.Next()

	// 8. 如果需要加密，则加密响应内容
	responseBody := bodyWriter.buf.Bytes()
	respContentType := c.Writer.Header().Get("Content-Type")
	if d.isPathEncrypted(c.Request.URL.Path) {
		if cryptor == nil {
			panic("cryptor is nil")
		}
		var err error
		responseBody, err = cryptor.Encrypt(responseBody)
		if err != nil {
			c.AbortWithError(500, errors.New("encrypt fail"))
			return
		}
		respContentType = ContentTypeEncrypted
	}

	// 9. 生成响应签名
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
	respSignature, err := signer.Sign(respSignData)
	if err != nil {
		c.AbortWithError(500, errors.New("sign fail"))
	}
	// 10. 写入响应头
	c.Header("X-Signature", respTimestamp)
	c.Header("X-Nonce", respNonce)
	c.Header("X-Timestamp", string(respSignature))
	// browser need this, or it cannot read these headers
	c.Header("Access-Control-Expose-Headers", "X-Timestamp,X-Nonce,X-Signature")
	c.Header("Content-Type", respContentType)
	c.Header("Content-Length", strconv.Itoa(len(responseBody)))
	c.Writer.Write(responseBody)
}

func (d *Authenticator) checkReplay(c *gin.Context, nonce, timestamp string) (canContinue bool) {
	timestampVal, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		c.AbortWithError(400, errors.New("timestamp is invalid"))
		return false
	}
	if time.Now().Unix()-timestampVal > 300 {
		c.AbortWithError(400, errors.New("repeat request"))
		return false // 超过5分钟的请求视为无效
	}
	res, err := d.redisClient.SetNX(c, "reply_check:"+nonce, "1", time.Duration(300)*time.Second).Result()
	if err != nil {
		c.AbortWithStatus(500)
		return false
	}
	if !res {
		// 重复请求
		c.AbortWithError(400, errors.New("repeat request"))
		return false
	}
	return true
}

func (d *Authenticator) verifyToken(c *gin.Context) (canContinue bool) {
	tokenString := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	if d.isPathNeedAuth(c.Request.URL.Path) {
		if len(tokenString) == 0 {
			c.AbortWithStatus(401)
			return false
		}
		revoked, err := d.IsTokenRevoked(c, tokenString)
		if err != nil {
			c.AbortWithError(500, errors.New("check token revoke fail"))
			return false
		}
		if revoked {
			c.AbortWithStatus(401)
			return false
		}
		// 解析Token
		claims, err := d.parseToken(tokenString)
		if err != nil {
			c.AbortWithError(401, errors.New("invalid token"))
			return false
		}
		c.Set("claims", claims)
	} else if len(tokenString) > 0 {
		revoked, _ := d.IsTokenRevoked(c, tokenString)
		if !revoked {
			// 解析Token
			claims, err := d.parseToken(tokenString)
			if err == nil {
				// 忽略错误
				c.Set("claims", claims)
			}
		}
	}
	return true
}

func (d *Authenticator) replaceRequestBody(c *gin.Context, reqBody []byte) (canContinue bool, cryptor Cryptor) {
	if len(reqBody) == 0 {
		return true, nil
	}

	if d.isPathEncrypted(c.Request.URL.Path) {
		if d.cryptorResolver == nil {
			panic("cryptor is nil")
		}
		contentType := c.GetHeader("Content-Type")
		if !strings.EqualFold(contentType, ContentTypeEncrypted) {
			c.AbortWithStatus(400)
			return false, cryptor
		}
		var err error
		cryptor, err = d.cryptorResolver.Resolve(c)
		if err != nil {
			c.AbortWithError(500, err)
			return false, cryptor
		}
		// 解密
		reqBody, err = cryptor.Decrypt(reqBody)
		if err != nil {
			c.AbortWithError(400, errors.New("decrypt fail"))
			return false, cryptor
		}

		c.Request.Header.Set("Content-Type", d.getDecryptContentType(c.Request.URL.Path))
	}

	buf := d.getBuffer()
	buf.Write(reqBody)
	c.Request.Body = io.NopCloser(buf)
	c.Request.ContentLength = int64(len(reqBody))

	return true, cryptor
}

func (a *Authenticator) parseToken(tokenString string) (*CustomClaims, error) {
	if len(a.jwtSecret) == 0 {
		panic("jwtSecret is empty")
	}
	token, err := jwt.ParseWithClaims(
		tokenString,
		&CustomClaims{},
		func(token *jwt.Token) (any, error) {
			return a.jwtSecret, nil // 返回用于验证签名的密钥
		},
	)
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil // 验证通过后返回自定义声明数据
	}
	return nil, err
}

func (a *Authenticator) GenerateToken(userID int, role, platform string) (string, error) {
	if len(a.jwtSecret) == 0 {
		panic("jwtSecret is empty")
	}
	claims := CustomClaims{
		UserId:   userID,
		Role:     role,
		Platform: platform,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.jwtTokenTTL)), // 过期时间
			Issuer:    a.jwtIssuer,                                       // 签发者
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtSecret) // 使用 HMAC-SHA256 算法签名
}

func (a *Authenticator) RevokeToken(ctx context.Context, token string) error {
	// 将Token添加到Redis集合中，表示已吊销
	err := a.redisClient.SAdd(ctx, "revoked_tokens", token).Err()
	return err
}

func (a *Authenticator) IsTokenRevoked(ctx context.Context, token string) (bool, error) {
	// 检查Token是否存在于Redis集合中
	exists, err := a.redisClient.SIsMember(ctx, "revoked_tokens", token).Result()
	if err != nil {
		return false, err
	}
	return exists, nil
}

// 自定义响应写入器
type bodyWriter struct {
	gin.ResponseWriter
	buf *bytes.Buffer
}

func (w bodyWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

type CustomClaims struct {
	UserId               int    `json:"u"`
	Role                 string `json:"r"`
	Platform             string `json:"p"`
	jwt.RegisteredClaims        // 包含标准字段如 exp（过期时间）、iss（签发者）等
}
