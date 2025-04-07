package niu

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// 客户端连接的消息
type LineMessage struct {
	UserId   string
	Platform Platform
	LineId   string
	Data     []byte
}

// 客户端连接的错误
type LineError struct {
	UserId   string
	Platform Platform
	LineId   string
	Error    error
}

// 客户端连接
type Line struct {
	hub        *Hub
	conn       *websocket.Conn
	id         string
	userId     string
	platform   Platform
	lastActive int64
	closeChan  chan Empty
	writeChan  chan []byte
}

func (ln *Line) Id() string { return ln.id }

func (ln *Line) UserId() string { return ln.userId }

func (ln *Line) Platform() Platform { return ln.platform }

func (ln *Line) LastActive() int64 { return atomic.LoadInt64(&ln.lastActive) }

func (ln *Line) Hub() *Hub { return ln.hub }

func (ln *Line) start() error {
	err := ln.hub.pool.Submit(func() {
		ln.conn.SetPingHandler(func(appData string) error {
			atomic.StoreInt64(&ln.lastActive, time.Now().Unix())
			return ln.conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(ln.hub.writeTimeout))
		})
		for {
			err := ln.conn.SetReadDeadline(time.Now().Add(ln.hub.readTimeout))
			if err != nil {
				ln.close(false, err)
				break
			}

			msgType, r, err := ln.conn.NextReader()
			if err != nil {
				ln.close(false, err)
				break
			}
			switch msgType {
			case websocket.CloseMessage:
				ln.close(false, nil)
				return
			case websocket.TextMessage:
				ln.close(true, nil) // 不允许文本消息
				return
			}

			// 池化读缓冲，提高性能
			buf := ln.hub.readBufferPool.Get()
			defer ln.hub.readBufferPool.Put(buf)
			_, err = io.Copy(buf, r)
			if err != nil {
				ln.close(false, err)
				return
			}

			atomic.StoreInt64(&ln.lastActive, time.Now().Unix())
			ln.hub.messageChan <- &LineMessage{ln.userId, ln.platform, ln.id, buf.Bytes()}
		}
	})
	if err != nil {
		ln.conn.Close()
		return err
	}

	err = ln.hub.pool.Submit(func() {
		for {
			select {
			case msg := <-ln.writeChan:
				err = ln.conn.SetWriteDeadline(time.Now().Add(ln.hub.writeTimeout))
				if err != nil {
					ln.close(false, err)
				}
				err = ln.conn.WriteMessage(websocket.BinaryMessage, msg)
				if err != nil {
					ln.close(false, err)
				}
			case <-ln.closeChan:
				ln.close(true, nil)
				return
			}
		}
	})
	if err != nil {
		ln.conn.Close()
		return err
	}

	ln.hub.registeredChan <- ln
	return nil
}

func (ln *Line) close(sendCloseCtrl bool, err error) {
	if err != nil {
		ln.hub.errorChan <- &LineError{ln.userId, ln.platform, ln.id, err}
	}

	if sendCloseCtrl {
		// 需要调用以下消息发送关闭消息，这样客户端才能正确识别关闭代码
		// 否则会导致客户端一直重连
		// 不能调用 s.Conn.Close()
		message := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		ln.conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(ln.hub.writeTimeout))
	}
	ln.conn.Close()
	ln.hub.unregisteredChan <- ln
}

// 用户在各个平台的所有连接
type UserLines struct {
	sync.RWMutex
	lines []*Line
}

// 获取连接数量
func (u *UserLines) Len() int {
	u.RLock()
	defer u.RUnlock()
	return len(u.lines)
}

// 添加连接
func (u *UserLines) add(line *Line) {
	u.Lock()
	defer u.Unlock()

	u.lines = append(u.lines, line)
}

// 关闭指定连接
func (u *UserLines) Close(lineId string) {
	u.Lock()
	defer u.Unlock()
	lines := make([]*Line, 0)
	for _, v := range u.lines {
		if v.id == lineId {
			v.closeChan <- Empty{}
		} else {
			lines = append(lines, v)
		}
	}
	u.lines = lines
}

// 获取指定连接
func (u *UserLines) Get(lineId string) *Line {
	u.RLock()
	defer u.RUnlock()

	for _, v := range u.lines {
		if v.id == lineId {
			return v
		}
	}
	return nil
}

// 获取指定平台的所有连接
func (u *UserLines) GetPlatformLines(platforms ...Platform) []*Line {
	if len(platforms) == 0 {
		return nil
	}

	u.RLock()
	defer u.RUnlock()

	lines := make([]*Line, 0)
	for _, v := range u.lines {
		if slices.Contains(platforms, v.platform) {
			lines = append(lines, v)
		}
	}
	return lines
}

// 关闭指定平台的所有连接
func (u *UserLines) ClosePlatforms(platforms ...Platform) {
	if len(platforms) == 0 {
		return
	}

	u.Lock()
	defer u.Unlock()

	lines := make([]*Line, 0)
	for _, line := range u.lines {
		if slices.Contains(platforms, line.platform) {
			line.closeChan <- Empty{}
		} else {
			lines = append(lines, line)
		}
	}
	u.lines = lines
}

// 关闭除指定平台外的所有连接
func (u *UserLines) ClosePlatformsExcept(exceptPlatforms ...Platform) {
	if len(exceptPlatforms) == 0 {
		return
	}

	u.Lock()
	defer u.Unlock()

	lines := make([]*Line, 0)
	for _, line := range u.lines {
		if slices.Contains(exceptPlatforms, line.platform) {
			lines = append(lines, line)
		} else {
			line.closeChan <- Empty{}
		}
	}
	u.lines = lines
}

// 关闭指定连接
func (u *UserLines) CloseLines(lineIds ...string) {
	if len(lineIds) == 0 {
		return
	}

	u.Lock()
	defer u.Unlock()

	lines := make([]*Line, 0)
	for _, line := range u.lines {
		if slices.Contains(lineIds, line.id) {
			line.closeChan <- Empty{}
		} else {
			lines = append(lines, line)
		}
	}
	u.lines = lines
}

// 关闭除指定连接外的所有连接
func (u *UserLines) CloseLinesExcept(exceptLineIds ...string) {
	if len(exceptLineIds) == 0 {
		return
	}

	u.Lock()
	defer u.Unlock()

	lines := make([]*Line, 0)
	for _, line := range u.lines {
		if slices.Contains(exceptLineIds, line.id) {
			lines = append(lines, line)
		} else {
			line.closeChan <- Empty{}
		}
	}
	u.lines = lines
}

// 关闭所有超过指定时间未活跃的连接
func (u *UserLines) closeInactiveLines(maxIdleSeconds int64) {
	if maxIdleSeconds <= 0 {
		return
	}

	u.Lock()
	defer u.Unlock()

	lines := make([]*Line, 0)
	for _, line := range u.lines {
		if time.Now().Unix()-line.lastActive > maxIdleSeconds {
			line.closeChan <- Empty{}
		} else {
			lines = append(lines, line)
		}
	}
	u.lines = lines
}

// 关闭所有连接
func (u *UserLines) CloseAll() {
	u.Lock()
	defer u.Unlock()

	for _, line := range u.lines {
		line.closeChan <- Empty{}
	}
	u.lines = make([]*Line, 0)
}

// 向该用户的所有连接发送消息
func (u *UserLines) PushMessage(data []byte) {
	if len(data) == 0 {
		return
	}

	u.RLock()
	defer u.RUnlock()

	for _, line := range u.lines {
		line.writeChan <- data
	}
}

// 向该用户的所有连接发送消息，除了指定平台
func (u *UserLines) PushMessageExceptPlatforms(data []byte, exceptPlatforms ...Platform) {
	if len(exceptPlatforms) == 0 || len(data) == 0 {
		return
	}

	u.RLock()
	defer u.RUnlock()

	for _, line := range u.lines {
		if slices.Contains(exceptPlatforms, line.platform) {
			continue
		}
		line.writeChan <- data
	}
}

// 向该用户的所有连接发送消息，除了指定连接
func (u *UserLines) PushMessageExceptLines(data []byte, exceptLineIds ...string) {
	if len(exceptLineIds) == 0 || len(data) == 0 {
		return
	}

	u.RLock()
	defer u.RUnlock()

	for _, line := range u.lines {
		if slices.Contains(exceptLineIds, line.id) {
			continue
		}
		line.writeChan <- data
	}
}

// 向该用户的指定平台发送消息
func (u *UserLines) PushMessageToPlatforms(data []byte, platforms ...Platform) {
	if len(platforms) == 0 || len(data) == 0 {
		return
	}

	u.RLock()
	defer u.RUnlock()

	for _, line := range u.lines {
		if slices.Contains(platforms, line.platform) {
			line.writeChan <- data
		}
	}
}

// 向该用户的指定连接发送消息
func (u *UserLines) PushMessageToLines(data []byte, lineIds ...string) {
	if len(lineIds) == 0 || len(data) == 0 {
		return
	}

	u.RLock()
	defer u.RUnlock()

	for _, line := range u.lines {
		if slices.Contains(lineIds, line.id) {
			line.writeChan <- data
		}
	}
}

type Hub struct {
	connections sync.Map // key: userId , value: UserLines

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

	messageChan      chan *LineMessage
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
		messageChan:        make(chan *LineMessage, 4096),
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
			delArr := make([]string, 0)
			h.connections.Range(func(key, value any) bool {
				conn := value.(*UserLines)
				conn.closeInactiveLines(h.connMaxIdleSeconds)
				if conn.Len() == 0 {
					delArr = append(delArr, key.(string))
				}
				return true
			})

			for _, v := range delArr {
				h.connections.Delete(v)
			}
		}
	})
	if err != nil {
		return nil, err
	}
	// 新的连接加入
	err = h.pool.Submit(func() {
		for ln := range h.registeredChan {
			ln.closeChan = make(chan Empty)
			ln.writeChan = make(chan []byte, 2048)
			// 新的连接加入
			lines, _ := h.connections.LoadOrStore(ln.userId, &UserLines{lines: []*Line{}})
			lines.(*UserLines).add(ln)
			h.connCount.Add(1)
		}
	})
	if err != nil {
		return nil, err
	}
	// 连接断开
	err = h.pool.Submit(func() {
		for ln := range h.unregisteredChan {
			// 连接断开
			lines, ok := h.connections.Load(ln.userId)
			if ok {
				lines.(*UserLines).Close(ln.id)
			}
			h.connCount.Add(-1)
			// 先删后关，防止在关闭之后，出现向通道意外发送的情况
			close(ln.closeChan)
			close(ln.writeChan)

			// 如果用户没有连接，则删除用户
			if ok && lines.(*UserLines).Len() == 0 {
				h.connections.Delete(ln.userId)
			}
		}
	})
	if err != nil {
		return nil, err
	}

	return h, nil
}

// 返回只读通道
func (h *Hub) MessageChan() <-chan *LineMessage { return h.messageChan }

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
		value.(*UserLines).CloseAll()
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

// 获取指定用户的所有连接
func (h *Hub) GetUserLines(userId string) *UserLines {
	lines, ok := h.connections.Load(userId)
	if !ok {
		return nil
	}
	return lines.(*UserLines)
}

// 关闭指定用户的所有连接
func (h *Hub) CloseUserLines(userIds ...string) {
	if len(userIds) == 0 {
		return
	}

	for _, userId := range userIds {
		if len(userId) == 0 {
			continue
		}
		lines, ok := h.connections.Load(userId)
		if !ok {
			continue
		}
		lines.(*UserLines).CloseAll()
	}
}

func (h *Hub) PushMessage(userIds []string, data []byte) {
	if len(userIds) == 0 || len(data) == 0 {
		return
	}
	h.pool.Submit(func() {
		for _, userId := range userIds {
			lines, ok := h.connections.Load(userId)
			if !ok {
				continue
			}
			lines.(*UserLines).PushMessage(data)
		}
	})
}

func (h *Hub) BroadcastMessage(data []byte) {
	if len(data) == 0 {
		return
	}
	h.pool.Submit(func() {
		h.connections.Range(func(key, lns any) bool {
			lns.(*UserLines).PushMessage(data)
			return true
		})
	})
}

func (h *Hub) UpgradeWebSocket(userId string, platform Platform, lineId string, w http.ResponseWriter, r *http.Request) error {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}

	// 存下该平台新的连接
	ln := &Line{
		hub:        h,
		conn:       conn,
		userId:     userId,
		platform:   Platform(platform),
		id:         lineId,
		lastActive: time.Now().Unix(),
	}

	// 开始监听该连接的消息
	return ln.start()
}
