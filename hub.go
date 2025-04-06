package niu

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

func hubKey(userId string, platform Platform) string {
	return fmt.Sprintf("%s:%d", userId, platform)
}

type HubMessage struct {
	UserId   string
	Platform Platform
	Data     []byte
}

type LineError struct {
	UserId   string
	Platform Platform
	Error    error
}

type Line struct {
	hub        *Hub
	conn       *websocket.Conn
	userId     string
	platform   Platform
	lastActive int64
	closeChan  chan Empty
	writeChan  chan []byte
}

func (c *Line) UserId() string { return c.userId }

func (c *Line) Platform() Platform { return c.platform }

func (c *Line) start() error {
	err := c.hub.pool.Submit(func() {
		c.conn.SetPingHandler(func(appData string) error {
			atomic.StoreInt64(&c.lastActive, time.Now().Unix())
			return c.conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(c.hub.writeTimeout))
		})
		for {
			err := c.conn.SetReadDeadline(time.Now().Add(c.hub.readTimeout))
			if err != nil {
				c.close(false, err)
				break
			}

			msgType, r, err := c.conn.NextReader()
			if err != nil {
				c.close(false, err)
				break
			}
			switch msgType {
			case websocket.CloseMessage:
				c.close(false, nil)
				break
			case websocket.TextMessage:
				c.close(true, nil) // 不允许文本消息
				break
			}

			// 池化读缓冲，提高性能
			buf := c.hub.readBufferPool.Get()
			defer c.hub.readBufferPool.Put(buf)
			_, err = io.Copy(buf, r)
			if err != nil {
				c.close(false, err)
				break
			}

			atomic.StoreInt64(&c.lastActive, time.Now().Unix())
			c.hub.messageChan <- &HubMessage{c.userId, c.platform, buf.Bytes()}
		}
	})
	if err != nil {
		c.conn.Close()
		return err
	}

	err = c.hub.pool.Submit(func() {
		for {
			select {
			case msg := <-c.writeChan:
				err = c.conn.SetWriteDeadline(time.Now().Add(c.hub.writeTimeout))
				if err != nil {
					c.close(false, err)
				}
				err = c.conn.WriteMessage(websocket.BinaryMessage, msg)
				if err != nil {
					c.close(false, err)
				}
			case <-c.closeChan:
				c.close(true, nil)
				return
			}
		}
	})
	if err != nil {
		c.conn.Close()
		return err
	}

	c.hub.registeredChan <- c
	return nil
}

func (c *Line) close(sendCloseCtrl bool, err error) {
	if err != nil {
		c.hub.errorChan <- &LineError{c.userId, c.platform, err}
	}

	if sendCloseCtrl {
		// 需要调用以下消息发送关闭消息，这样客户端才能正确识别关闭代码
		// 否则会导致客户端一直重连
		// 不能调用 s.Conn.Close()
		message := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		c.conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(c.hub.writeTimeout))
	}
	c.conn.Close()
	c.hub.unregisteredChan <- c
}

type Hub struct {
	connections sync.Map // key: userId:platform, value: Connection

	subprotocols []string
	connCount    atomic.Int32 // 所有仍在连接状态的数量
	pool         CoroutinePool

	liveCheckDuration  time.Duration
	liveTicker         *time.Ticker
	connMaxIdleSeconds int64
	upgrader           websocket.Upgrader

	writeTimeout   time.Duration
	readTimeout    time.Duration
	readBufferPool *ByteBufferPool

	messageChan      chan *HubMessage
	registeredChan   chan *Line
	unregisteredChan chan *Line
	errorChan        chan *LineError
}

func NewHub(
	subprotocols []string,
	liveCheckDuration, connMaxIdleTime,
	readTimeout, writeTimeout time.Duration,
	pool CoroutinePool,
	handshakeTimeout time.Duration,
	enableCompression bool,
	checkOriginFn func(r *http.Request) bool,
) (*Hub, error) {
	if pool == nil {
		return nil, errors.New("pool must not nil")
	}
	if liveCheckDuration < time.Second {
		liveCheckDuration = time.Second
	}

	h := &Hub{
		connections:        sync.Map{},
		subprotocols:       subprotocols,
		pool:               pool,
		liveCheckDuration:  liveCheckDuration,
		readTimeout:        readTimeout,
		writeTimeout:       writeTimeout,
		connMaxIdleSeconds: int64(connMaxIdleTime),
		liveTicker:         time.NewTicker(liveCheckDuration),
		readBufferPool:     NewByteBufferPool(0, 2048),
		messageChan:        make(chan *HubMessage, 4096),
		registeredChan:     make(chan *Line, 2048),
		unregisteredChan:   make(chan *Line, 2048),
		errorChan:          make(chan *LineError, 2048),
		upgrader: websocket.Upgrader{
			EnableCompression: enableCompression,
			HandshakeTimeout:  handshakeTimeout,
			ReadBufferSize:    4096,
			WriteBufferSize:   4096,
			Subprotocols:      subprotocols,
			WriteBufferPool:   &sync.Pool{},
			CheckOrigin:       checkOriginFn,
		},
	}

	// 检测连接可用性
	err := h.pool.Submit(func() {
		ticker := h.liveTicker
		for range ticker.C {
			delArr := make([]*Line, 0)
			h.connections.Range(func(key, value any) bool {
				conn := value.(*Line)
				if time.Now().Unix()-atomic.LoadInt64(&conn.lastActive) > h.connMaxIdleSeconds {
					delArr = append(delArr, conn)
				}
				return true
			})

			for _, v := range delArr {
				v.closeChan <- Empty{}
			}
		}
	})
	if err != nil {
		return nil, err
	}
	// 新的连接加入
	err = h.pool.Submit(func() {
		for conn := range h.registeredChan {
			conn.closeChan = make(chan Empty)
			conn.writeChan = make(chan []byte, 2048)
			// 新的连接加入
			h.connections.Store(hubKey(conn.userId, conn.platform), conn)
			h.connCount.Add(1)
		}
	})
	if err != nil {
		return nil, err
	}
	// 连接断开
	err = h.pool.Submit(func() {
		for conn := range h.unregisteredChan {
			// 连接断开
			h.connections.Delete(hubKey(conn.userId, conn.platform))
			h.connCount.Add(-1)
			// 先删后关，防止在关闭之后，出现向通道意外发送的情况
			close(conn.closeChan)
			close(conn.writeChan)
		}
	})
	if err != nil {
		return nil, err
	}

	return h, nil
}

// 返回只读通道
func (h *Hub) MessageChan() <-chan *HubMessage { return h.messageChan }

func (h *Hub) RegisteredChan() <-chan *Line { return h.registeredChan }

func (h *Hub) UnegisteredChan() <-chan *Line { return h.unregisteredChan }

func (h *Hub) ErrorChan() <-chan *LineError { return h.errorChan }

func (h *Hub) LiveCount() int { return int(h.connCount.Load()) }

func (h *Hub) Close(wait time.Duration) {
	if h.liveTicker != nil {
		h.liveTicker.Stop()
		h.liveTicker = nil
	}
	h.connections.Range(func(key, value any) bool {
		value.(*Line).closeChan <- Empty{}
		return true
	})

	time.Sleep(wait)
	defer func() {
		r := recover()
		if r != nil {
			fmt.Println("close chan err")
		}
	}()

	close(h.messageChan)
	close(h.registeredChan)
	close(h.unregisteredChan)
	close(h.errorChan)

	h.readBufferPool = nil
	h.connections.Clear()
}

// platform == Unspecify 时表示关闭该用户的所有连接
func (h *Hub) CloseLine(userId string, platform Platform) {
	if len(userId) == 0 {
		return
	}

	if platform == Unspecify {
		for _, p := range Platforms {
			conn, ok := h.connections.Load(hubKey(userId, p))
			if !ok {
				continue
			}
			conn.(*Line).closeChan <- Empty{}
		}
	} else {
		conn, ok := h.connections.Load(hubKey(userId, platform))
		if !ok {
			return
		}
		conn.(*Line).closeChan <- Empty{}
	}
}

func (h *Hub) CloseLineExcept(userId string, exceptPlatform Platform) {
	if len(userId) == 0 {
		return
	}

	for _, p := range Platforms {
		if p == exceptPlatform {
			continue // 该连接不关闭
		}
		conn, ok := h.connections.Load(hubKey(userId, p))
		if !ok {
			continue
		}
		conn.(*Line).closeChan <- Empty{}
	}
}

func (h *Hub) PushMessage(userIds []string, data []byte) {
	if len(userIds) == 0 || len(data) == 0 {
		return
	}
	h.pool.Submit(func() {
		for _, userId := range userIds {
			for _, p := range Platforms {
				conn, ok := h.connections.Load(hubKey(userId, p))
				if !ok {
					continue
				}
				conn.(*Line).writeChan <- data
			}
		}
	})
}

func (h *Hub) BroadcastMessage(data []byte) {
	if len(data) == 0 {
		return
	}
	h.pool.Submit(func() {
		h.connections.Range(func(key, pcs any) bool {
			pcs.(*Line).writeChan <- data
			return true
		})
	})
}

func (h *Hub) UpgradeWebSocket(userId string, platform Platform, w http.ResponseWriter, r *http.Request) error {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}

	existCon, ok := h.connections.Load(hubKey(userId, Platform(platform)))
	if ok {
		// 已经存在该平台的连接，关闭该连接
		existCon.(*Line).closeChan <- Empty{}
	}
	// 存下该平台新的连接
	cc := &Line{
		conn:       conn,
		userId:     userId,
		platform:   Platform(platform),
		lastActive: time.Now().Unix(),
	}

	// 开始监听该连接的消息
	return cc.start()
}
