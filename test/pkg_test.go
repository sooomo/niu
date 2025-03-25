package test

import (
	"fmt"
	"net/http"
	"niu/core"
	"niu/net"
	"niu/net/protocols"
	"testing"
	"time"

	"niu/crypto"

	"github.com/gin-gonic/gin"
	"github.com/panjf2000/ants/v2"
)

var hub *net.Hub

func GetHub() *net.Hub { return hub }

func Start() {
	net.InitSignHeaders("niu")
	p, _ := ants.NewPool(10000)
	hub, _ = net.NewHub(
		[]string{"niu-v1"},
		2*time.Minute,
		time.Minute,
		30*time.Second,
		30*time.Second,
		p,
		10*time.Second,
		false,
		func(r *http.Request) bool { return true },
	)
	msgProto := protocols.NewMsgPackProtocol(nil, nil)
	p.Submit(func() {
		for {
			select {
			case msg, ok := <-hub.MessageChan():
				if ok {
					var mp map[string]any
					if msgType, err := msgProto.DecodeReq(msg.Data, &mp); err == nil {
						fmt.Printf("recv msg type:%v, val is:%v", msgType, mp)
					}
				}
			case r, ok := <-hub.RegisteredChan():
				if ok {
					fmt.Printf("line registered: userid->%v, platform->%v", r.UserId(), r.Platform())
				}
			case u, ok := <-hub.UnegisteredChan():
				if ok {
					fmt.Printf("line unregistered: userid->%v, platform->%v", u.UserId(), u.Platform())
				}
			case e, ok := <-hub.ErrorChan():
				if ok {
					fmt.Printf("line error: userid->%v, platform->%v, err:%v", e.UserId, e.Platform, e.Error)
				}
			default:
				continue
			}
		}
	})
}

func UpgradeWebSocket(ctx *gin.Context) {
	userId := ctx.GetString("user_id")
	platform := ctx.GetInt("platform")
	err := hub.UpgradeWebSocket(
		userId,
		core.Platform(platform),
		ctx.Writer,
		ctx.Request,
	)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
	}
}

func TestPKG(t *testing.T) {

	Start()

	r := gin.Default()

	r.Use(net.ReplayInterceptMiddleware(func(reqId string) bool {
		return true
	}))
	// ----长连接请求-----ws连接也可以指定 nonce 头
	r.GET("/chat", UpgradeWebSocket)
	//---------------以下为http请求-------------
	r.Use(net.SignatureMiddleware(func(ctx *gin.Context) crypto.Signer {
		return nil
	}, net.DefaultSignRule))
	r.Use(net.CryptoMiddleware(func(ctx *gin.Context) crypto.Cryptor {
		return nil
	}))

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	r.Run()
}
