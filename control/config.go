package control

import (
	"errors"
	"net"
	"sync"

	"github.com/mia0x75/nova/app"
	"github.com/mia0x75/nova/services"
)

const (
	CMD_ERROR = iota // 错误响应
	CMD_TICK         // 心跳包
	CMD_STOP
	CMD_RELOAD
	CMD_SHOW_MEMBERS
)

const (
	tcpDefaultReadBufferSize = 1024
)

const (
	tcpNodeOnline = 1 << iota
	tcpNodeIsControl
)

type TcpClientNode struct {
	conn       *net.Conn // 客户端连接进来的资源句柄
	recvBuf    []byte    // 读缓冲区
	status     int
	wg         *sync.WaitGroup
	ctx        *app.Context
	lock       *sync.Mutex // 互斥锁，修改资源时锁定
	stop       StopFunc
	reload     ReloadFunc
	showmember ShowMemberFunc
}

type TcpService struct {
	Address    string
	lock       *sync.Mutex
	ctx        *app.Context
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
	nodeOffline    = errors.New("tcp node offline")
)

type ShowMemberFunc func() string
type ReloadFunc func(service string)
type StopFunc func()
type ControlOption func(tcp *TcpService)
type nodeOption func(node *TcpClientNode)
