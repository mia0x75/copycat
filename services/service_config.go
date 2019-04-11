package services

import (
	"net"
	"sync"

	"github.com/mia0x75/copycat/g"
)

// IService 服务接口
type IService interface {
	SendAll(table string, data []byte) bool // 服务广播
	Start()                                 // 启动服务
	Close()                                 // 关闭服务
	Reload()                                // 重新加载服务配置
	Name() string                           // 返回服务名称
}

const (
	CMD_SET_PRO = iota // 注册客户端操作，加入到指定分组
	CMD_AUTH           // 认证（暂未使用）
	CMD_ERROR          // 错误响应
	CMD_TICK           // 心跳包
	CMD_EVENT          // 事件
	CMD_AGENT
	CMD_STOP
	CMD_RELOAD
	CMD_SHOW_MEMBERS
	CMD_POS
)

const (
	tcpMaxSendQueue          = 10000000
	httpMaxSendQueue         = 10000000
	tcpDefaultReadBufferSize = 1024
)

const (
	// FlagSetPro TODO
	FlagSetPro = iota
	// FlagPing TODO
	FlagPing
)

const (
	serviceEnable = 1 << iota
	serviceClosed
)

const (
	tcpNodeOnline = 1 << iota
)

// ServiceName TODO
const ServiceName = "binlog-go-subscribe"

type tcpClientNode struct {
	conn             *net.Conn       // 客户端连接进来的资源句柄
	sendQueue        chan []byte     // 发送channel
	sendFailureTimes int64           // 发送失败次数
	topics           []string        // 订阅的主题
	recvBuf          []byte          // 读缓冲区
	connectTime      int64           // 连接成功的时间戳
	status           int             //
	wg               *sync.WaitGroup //
	ctx              *g.Context      //
	lock             *sync.Mutex     // 互斥锁，修改资源时锁定
	onclose          []NodeFunc      //
}

// NodeFunc TODO
type NodeFunc func(n *tcpClientNode)

// SetProFunc TODO
type SetProFunc func(n *tcpClientNode, groupName string) bool

// NodeOption TODO
type NodeOption func(n *tcpClientNode)

type tcpGroup struct {
	name   string
	filter []string
	nodes  []*tcpClientNode
	lock   *sync.Mutex
}

// TCPService TODO
type TCPService struct {
	IService
	Listen      string // 监听ip
	lock        *sync.Mutex
	statusLock  *sync.Mutex
	ctx         *g.Context
	listener    *net.Listener
	wg          *sync.WaitGroup
	status      int
	conn        *net.TCPConn
	buffer      []byte
	sendAll     []SendAllFunc
	sendRaw     []SendRawFunc
	onConnect   []OnConnectFunc
	onClose     []CloseFunc
	onKeepalive []KeepaliveFunc
	reload      []ReloadFunc
}

var (
	_ IService = &TCPService{}

	packDataTickOk = Pack(CMD_TICK, []byte("ok"))
	packDataSetPro = Pack(CMD_SET_PRO, []byte("ok"))
)

// TCPServiceOption TODO
type TCPServiceOption func(service *TCPService)

// SendAllFunc TODO
type SendAllFunc func(table string, data []byte) bool

// SendRawFunc TODO
type SendRawFunc func(msg []byte)

// OnConnectFunc TODO
type OnConnectFunc func(conn *net.Conn)

// CloseFunc TODO
type CloseFunc func()

// KeepaliveFunc TODO
type KeepaliveFunc func(data []byte)

// ReloadFunc TODO
type ReloadFunc func()
