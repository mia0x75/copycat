package agent

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/g"
)

const (
	statusConnect     = 1 << iota
	asyncWriteChanLen = 10000
)

// Client TODO
type Client struct {
	ctx                 context.Context
	buffer              []byte
	bufferSize          int
	conn                net.Conn
	status              int
	onMessageCallback   []OnClientEventFunc
	asyncWriteChan      chan []byte
	coder               ICodec
	msgID               int64
	waiter              map[int64]*Waiter
	waiterLock          *sync.RWMutex
	waiterGlobalTimeout int64 //毫秒
}

// Waiter TODO
type Waiter struct {
	msgID     int64
	data      chan []byte
	time      int64
	delWaiter func(int64)
}

// Wait TODO
func (w *Waiter) Wait(timeout time.Duration) ([]byte, error) {
	a := time.After(timeout)
	select {
	case data, ok := <-w.data:
		if !ok {
			return nil, g.ErrChanIsClosed
		}
		msgID := int64(binary.LittleEndian.Uint64(data[:8]))
		w.delWaiter(msgID)
		return data[8:], nil
	case <-a:
		return nil, g.ErrWaitTimeout
	}
	// Unreachable code
	// return nil, g.ErrUnknownError
}

// ClientOption TODO
type ClientOption func(tcp *Client)

// OnClientEventFunc TODO
type OnClientEventFunc func(tcp *Client, content []byte)

// OnConnectFunc TODO
type OnConnectFunc func(tcp *Client)

// SetOnMessage 设置收到消息的回调函数
// 回调函数同步执行，不能使阻塞的函数
func SetOnMessage(f ...OnClientEventFunc) ClientOption {
	return func(tcp *Client) {
		tcp.onMessageCallback = append(tcp.onMessageCallback, f...)
	}
}

// SetCoder 用来设置编码解码的接口
func SetCoder(coder ICodec) ClientOption {
	return func(tcp *Client) {
		tcp.coder = coder
	}
}

// SetBufferSize 设置缓冲区大小
func SetBufferSize(size int) ClientOption {
	return func(tcp *Client) {
		tcp.bufferSize = size
	}
}

// SetWaiterGlobalTimeout 单位是毫秒
// 设置waiter检测的超时时间，默认为6000毫秒
// 如果超过该时间，waiter就会被删除
func SetWaiterGlobalTimeout(timeout int64) ClientOption {
	return func(tcp *Client) {
		tcp.waiterGlobalTimeout = timeout
	}
}

// NewClient TODO
func NewClient(ctx context.Context, opts ...ClientOption) *Client {
	c := &Client{
		buffer:              make([]byte, 0),
		conn:                nil,
		status:              0,
		onMessageCallback:   make([]OnClientEventFunc, 0),
		asyncWriteChan:      make(chan []byte, asyncWriteChanLen),
		ctx:                 ctx,
		coder:               &Codec{},
		bufferSize:          4096,
		msgID:               1,
		waiter:              make(map[int64]*Waiter),
		waiterLock:          new(sync.RWMutex),
		waiterGlobalTimeout: 6000,
	}
	for _, f := range opts {
		f(c)
	}
	go c.keep()
	go c.readMessage()
	return c
}

func (tcp *Client) delWaiter(msgID int64) {
	tcp.waiterLock.Lock()
	w, ok := tcp.waiter[msgID]
	if ok {
		close(w.data)
		delete(tcp.waiter, msgID)
	}
	tcp.waiterLock.Unlock()
}

// AsyncSend TODO
func (tcp *Client) AsyncSend(data []byte) {
	tcp.asyncWriteChan <- data
}

// Send TODO
func (tcp *Client) Send(data []byte) (*Waiter, int, error) {
	if tcp.status&statusConnect <= 0 {
		return nil, 0, g.ErrNotConnect
	}
	msgID := atomic.AddInt64(&tcp.msgID, 1)
	// check max msgID
	if msgID > math.MaxInt64 {
		atomic.StoreInt64(&tcp.msgID, 1)
		msgID = atomic.AddInt64(&tcp.msgID, 1)
	}
	wai := &Waiter{
		msgID:     msgID,
		data:      make(chan []byte, 1),
		time:      int64(time.Now().UnixNano() / 1000000),
		delWaiter: tcp.delWaiter,
	}
	fmt.Println("add waiter ", wai.msgID)
	tcp.waiterLock.Lock()
	tcp.waiter[wai.msgID] = wai
	tcp.waiterLock.Unlock()

	sendMsg := tcp.coder.Encode(msgID, data)
	num, err := tcp.conn.Write(sendMsg)
	return wai, num, err
}

// Write write api 与 send api的差别在于 send 支持同步wait等待服务端响应
// write 则不支持
func (tcp *Client) Write(data []byte) (int, error) {
	if tcp.status&statusConnect <= 0 {
		return 0, g.ErrNotConnect
	}
	msgID := atomic.AddInt64(&tcp.msgID, 1)
	// check max msgID
	if msgID > math.MaxInt64 {
		atomic.StoreInt64(&tcp.msgID, 1)
		msgID = atomic.AddInt64(&tcp.msgID, 1)
	}
	sendMsg := tcp.coder.Encode(msgID, data)
	num, err := tcp.conn.Write(sendMsg)
	return num, err
}

func (tcp *Client) keep() {
	go func() {
		for {
			tcp.Write(keepalivePackage)
			time.Sleep(time.Second * 3)
		}
	}()

	go func() {
		for {
			select {
			case sendData, ok := <-tcp.asyncWriteChan:
				//async send support
				if !ok {
					return
				}
				_, err := tcp.Write(sendData)
				if err != nil {
					log.Errorf("[E] send failure: %+v", err)
				}
			}
		}
	}()

	for {
		current := int64(time.Now().UnixNano() / 1000000)
		tcp.waiterLock.Lock()
		for msgID, v := range tcp.waiter {
			// check timeout
			if current-v.time >= tcp.waiterGlobalTimeout {
				log.Warnf("[W] msgid %v is timeout, will delete", msgID)
				close(v.data)
				delete(tcp.waiter, msgID)
			}
		}
		tcp.waiterLock.Unlock()
		time.Sleep(time.Second * 3)
	}
}

func (tcp *Client) readMessage() {
	for {
		if tcp.status&statusConnect <= 0 {
			time.Sleep(time.Millisecond * 100)
			continue
		}
		readBuffer := make([]byte, tcp.bufferSize)
		size, err := tcp.conn.Read(readBuffer)
		if err != nil || size <= 0 {
			log.Warnf("[W] client read with error: %+v", err)
			tcp.Disconnect()
			continue
		}
		tcp.onMessage(readBuffer[:size])
		select {
		case <-tcp.ctx.Done():
			return
		default:
		}
	}
}

// Connect use like go tcp.Connect()
func (tcp *Client) Connect(address string, timeout time.Duration) error {
	// 如果已经连接，直接返回
	if tcp.status&statusConnect > 0 {
		return g.ErrIsConnected
	}
	dial := net.Dialer{Timeout: timeout}
	conn, err := dial.Dial("tcp", address)
	if err != nil {
		log.Errorf("[E] start client with error: %+v", err)
		return err
	}
	if tcp.status&statusConnect <= 0 {
		tcp.status |= statusConnect
	}
	tcp.conn = conn
	return nil
}

func (tcp *Client) onMessage(msg []byte) {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("[E] onMessage recover%+v, %+v", err, tcp.buffer)
			tcp.buffer = make([]byte, 0)
		}
	}()
	tcp.buffer = append(tcp.buffer, msg...)
	for {
		bufferLen := len(tcp.buffer)
		msgID, content, pos, err := tcp.coder.Decode(tcp.buffer)
		if err != nil {
			log.Errorf("[E] %v", err)
			tcp.buffer = make([]byte, 0)
			return
		}
		if msgID <= 0 {
			return
		}
		if len(tcp.buffer) >= pos {
			tcp.buffer = append(tcp.buffer[:0], tcp.buffer[pos:]...)
		} else {
			tcp.buffer = make([]byte, 0)
			log.Errorf("[E] pos %v (olen=%v) error, content=%v(%v) len is %v, data is: %+v", pos, bufferLen, content, string(content), len(tcp.buffer), tcp.buffer)
		}
		// 1 is system id
		if msgID > 1 {
			data := make([]byte, 8+len(content))
			binary.LittleEndian.PutUint64(data[:8], uint64(msgID))
			copy(data[8:], content)
			tcp.waiterLock.RLock()
			w, ok := tcp.waiter[msgID]
			tcp.waiterLock.RUnlock()
			if ok {
				w.data <- data
			}
		}

		// 判断是否是心跳包，心跳包不触发回调函数
		if !bytes.Equal(keepalivePackage, content) {
			for _, f := range tcp.onMessageCallback {
				f(tcp, content)
			}
		}
	}
}

// Disconnect TODO
func (tcp *Client) Disconnect() {
	if tcp.status&statusConnect <= 0 {
		return
	}
	tcp.conn.Close()
	if tcp.status&statusConnect > 0 {
		tcp.status ^= statusConnect
	}
}
