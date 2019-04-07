package services

import (
	"net"
	"sync"
	"time"

	"github.com/mia0x75/nova/app"
)

type Service interface {
	SendAll(table string, data []byte) bool // 服务广播
	Start()                                 // 启动服务
	Close()                                 // 关闭服务
	Reload()                                // 重新加载服务配置
	Name() string                           // 返回服务名称
	SendRaw(data []byte) bool               // 用于发送agent透传的原始数据
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
	FlagSetPro = iota
	FlagPing
	FlagControl
	FlagAgent
)

const (
	serviceEnable = 1 << iota
)

type httpGroup struct {
	name   string    //
	filter []string  //
	nodes  httpNodes //
}

type httpNodes []*httpNode
type httpGroups map[string]*httpGroup

type HttpService struct {
	Service                //
	groups   httpGroups    //
	lock     *sync.Mutex   // 互斥锁，修改资源时锁定
	timeTick time.Duration // 故障检测的时间间隔
	ctx      *app.Context  // *context.Context
	status   int           //
}

type httpNode struct {
	url       string      // url
	sendQueue chan string // 发送channel
	lock      *sync.Mutex // 互斥锁，修改资源时锁定
	ctx       *app.Context
	wg        *sync.WaitGroup
}

const (
	tcpNodeOnline = 1 << iota
	tcpNodeIsNormal
	tcpNodeIsAgent
	tcpNodeIsControl
)

type tcpClientNode struct {
	conn             *net.Conn       // 客户端连接进来的资源句柄
	sendQueue        chan []byte     // 发送channel
	sendFailureTimes int64           // 发送失败次数
	group            string          // 所属分组
	recvBuf          []byte          // 读缓冲区
	connectTime      int64           // 连接成功的时间戳
	status           int             //
	wg               *sync.WaitGroup //
	ctx              *app.Context    //
	lock             *sync.Mutex     // 互斥锁，修改资源时锁定
	onclose          []NodeFunc
	onpro            SetProFunc
}

type NodeFunc func(n *tcpClientNode)
type SetProFunc func(n *tcpClientNode, groupName string) bool
type NodeOption func(n *tcpClientNode)

type tcpClients []*tcpClientNode
type tcpGroups map[string]*tcpGroup

type tcpGroup struct {
	name   string
	filter []string
	nodes  tcpClients
	lock   *sync.Mutex
}

type TcpService struct {
	Service
	Ip         string      // 监听ip
	Port       int         // 监听端口
	lock       *sync.Mutex // 互斥锁，修改资源时锁定
	statusLock *sync.Mutex
	groups     tcpGroups       //
	ctx        *app.Context    // *context.Context
	listener   *net.Listener   //
	wg         *sync.WaitGroup //
	ServiceIp  string
	status     int
	token      string
	conn       *net.TCPConn
	buffer     []byte
}

var (
	_ Service = &TcpService{}
	_ Service = &HttpService{}

	packDataTickOk = Pack(CMD_TICK, []byte("ok"))
	packDataSetPro = Pack(CMD_SET_PRO, []byte("ok"))
)
