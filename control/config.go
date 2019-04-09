package control

import (
	"errors"
	"net"
	"sync"

	"github.com/mia0x75/copycat/g"
	"github.com/mia0x75/copycat/services"
)

const (
	// CMD_ERROR 错误响应
	CMD_ERROR = iota
	// CMD_TICK 心跳包
	CMD_TICK
	// CMD_STOP TODO
	CMD_STOP
	// CMD_RELOAD TODO
	CMD_RELOAD
	// CMD_SHOW_MEMBERS TODO
	CMD_SHOW_MEMBERS
)

const (
	tcpDefaultReadBufferSize = 1024
)

const (
	tcpNodeOnline = 1 << iota
)

// TCPClientNode TODO
type TCPClientNode struct {
	conn       *net.Conn // 客户端连接进来的资源句柄
	recvBuf    []byte    // 读缓冲区
	status     int
	wg         *sync.WaitGroup
	ctx        *g.Context
	lock       *sync.Mutex // 互斥锁，修改资源时锁定
	stop       StopFunc
	reload     ReloadFunc
	showmember ShowMemberFunc
}

// TCPService TODO
type TCPService struct {
	Address    string
	lock       *sync.Mutex
	ctx        *g.Context
	listener   *net.Listener
	wg         *sync.WaitGroup
	token      string
	conn       *net.TCPConn
	buffer     []byte
	showmember ShowMemberFunc
	reload     ReloadFunc
	stop       StopFunc
}

var (
	packDataTickOk = services.Pack(CMD_TICK, []byte("ok"))
	errNodeOffline = errors.New("tcp node offline")
)

// ShowMemberFunc TODO
type ShowMemberFunc func() string

// ReloadFunc TODO
type ReloadFunc func(service string)

// StopFunc TODO
type StopFunc func()

// Option TODO
type Option func(tcp *TCPService)

// nodeOption TODO
type nodeOption func(node *TCPClientNode)
