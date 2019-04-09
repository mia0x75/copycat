package agent

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/consul"
	"github.com/mia0x75/copycat/g"
	"github.com/mia0x75/copycat/services"
	mtcp "github.com/mia0x75/copycat/tcp"
)

//agent 所需要做的事情

//如果当前的节点不是leader
//那么查询leader的agent服务ip以及端口
//所有非leader节点连接到leader节点
//如果pos改变，广播到所有的非leader节点上
//非leader节点保存pos信息

// todo 这里还需要一个异常检测机制
// 定期检测是否有leader在运行，如果没有，尝试强制解锁，然后选出新的leader
// ServiceName 服务名称
const ServiceName = "binlog-go-agent"

// 服务注册
const (
	Registered = 1 << iota
)

// OnLeaderFunc 回调
type OnLeaderFunc func(bool)

// OnEventFunc 回调
type OnEventFunc func(table string, data []byte) bool

// OnRawFunc 回调
type OnRawFunc func(msg []byte) bool

// TcpService 结构体
type TcpService struct {
	Address    string // 监听ip
	lock       *sync.Mutex
	statusLock *sync.Mutex
	ctx        *g.Context
	listener   *net.Listener
	wg         *sync.WaitGroup
	status     int
	conn       *net.TCPConn
	buffer     []byte
	client     *mtcp.Client
	enable     bool
	sService   consul.ILeader
	onLeader   []OnLeaderFunc
	leader     bool
	server     *mtcp.Server
	onEvent    []OnEventFunc
	onPos      []OnPosFunc
}

// NewAgentServer 创建实例
func NewAgentServer(ctx *g.Context, opts ...AgentServerOption) *TcpService {
	cfg := g.Config().Agent
	if !cfg.Enabled {
		s := &TcpService{
			enable: cfg.Enabled,
		}
		for _, f := range opts {
			f(s)
		}
		return s
	}
	tcp := &TcpService{
		Address:    cfg.Listen,
		lock:       new(sync.Mutex),
		statusLock: new(sync.Mutex),
		wg:         new(sync.WaitGroup),
		listener:   nil,
		ctx:        ctx,
		status:     0,
		buffer:     make([]byte, 0),
		enable:     cfg.Enabled,
		onEvent:    make([]OnEventFunc, 0),
		onPos:      make([]OnPosFunc, 0),
	}
	tcp.client = mtcp.NewClient(ctx.Ctx, mtcp.SetOnMessage(tcp.onClientMessage))
	// 服务注册
	strs := strings.Split(cfg.Listen, ":")
	ip := strs[0]
	port, _ := strconv.ParseInt(strs[1], 10, 32)

	tcp.sService = consul.NewLeader(
		cfg.Consul,
		cfg.Lock,
		ServiceName,
		ip,
		int(port),
	)
	for _, f := range opts {
		f(tcp)
	}
	tcp.server = mtcp.NewServer(ctx.Ctx, cfg.Listen, mtcp.SetOnServerMessage(tcp.onServerMessage))
	return tcp
}

// OnPos 设置收到pos的回调函数
func OnPos(f OnPosFunc) AgentServerOption {
	return func(s *TcpService) {
		if !s.enable {
			return
		}
		s.onPos = append(s.onPos, f)
	}
}

func OnLeader(f OnLeaderFunc) AgentServerOption {
	return func(s *TcpService) {
		if !s.enable {
			f(true)
			return
		}
		s.onLeader = append(s.onLeader, f)
	}
}

// OnEvent 事件回调
// 这个回调应该来源于service_plugin/tcp
// 最终被转发到SendAll
func OnEvent(f OnEventFunc) AgentServerOption {
	return func(s *TcpService) {
		if !s.enable {
			return
		}
		s.onEvent = append(s.onEvent, f)
	}
}

// OnRaw 原封不动转发到tcp SendRaw
func OnRaw(f OnRawFunc) AgentServerOption {
	return func(s *TcpService) {
		if !s.enable {
			return
		}
	}
}

func (tcp *TcpService) onClientMessage(client *mtcp.Client, content []byte) {
	cmd, data, err := services.Unpack(content)
	if err != nil {
		log.Error(err)
		return
	}
	switch cmd {
	case CMD_EVENT:
		var raw map[string]interface{}
		err = json.Unmarshal(data, &raw)
		if err == nil {
			table := raw["database"].(string) + "." + raw["table"].(string)
			for _, f := range tcp.onEvent {
				f(table, data)
			}
		}
	case CMD_POS:
		for _, f := range tcp.onPos {
			f(data)
		}
	}
}

func (tcp *TcpService) onServerMessage(node *mtcp.ClientNode, msgID int64, data []byte) {
}

// Start 启动
func (tcp *TcpService) Start() {
	if !tcp.enable {
		return
	}
	tcp.server.Start()
	tcp.sService.Select(func(member *consul.ServiceMember) {
		log.Infof("[I] current node %v is leader: %v", tcp.Address, member.IsLeader)
		tcp.leader = member.IsLeader
		for _, f := range tcp.onLeader {
			f(member.IsLeader)
		}
		// 连接到leader
		if !tcp.leader {
			for {
				m, err := tcp.sService.Get()
				if err == nil && m != nil {
					leaderAddress := fmt.Sprintf("%v:%v", m.ServiceIP, m.Port)
					log.Infof("[I] connect to leader %v", leaderAddress)
					tcp.client.Connect(leaderAddress, time.Second*3)
					break
				}
				log.Warnf("[W] leader is not init, try to wait init")
				time.Sleep(time.Second)
			}
		}
	})
}

// Close 关闭
func (tcp *TcpService) Close() {
	if !tcp.enable {
		return
	}
	log.Debugf("[D] tcp service closing, waiting for buffer send complete.")
	tcp.lock.Lock()
	defer tcp.lock.Unlock()
	if tcp.listener != nil {
		(*tcp.listener).Close()
	}
	log.Debugf("[D] tcp service closed.")
	tcp.server.Close()
	tcp.sService.Free()
}

// Sync 此api提供给binlog通过agent server同步广播发送给所有的client客户端
func (tcp *TcpService) Sync(data []byte) {
	if !tcp.enable {
		return
	}
	// 广播给agent client
	// agent client 再发送给连接到当前service_plugin/tcp的客户端
	tcp.server.Broadcast(1, data)
}

// ShowMembers 显示群集信息
func (tcp *TcpService) ShowMembers() string {
	if !tcp.enable {
		return "agent is not enable"
	}
	data, err := tcp.sService.GetServices(false) //.getMembers()
	if data == nil || err != nil {
		return ""
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}
	res := fmt.Sprintf("current node: %s(%s)\r\n", hostname, tcp.Address)
	res += fmt.Sprintf("cluster size: %d node(s)\r\n", len(data))
	res += fmt.Sprintf("======+=============================================+==========+===============\r\n")
	res += fmt.Sprintf("%-6s| %-43s | %-8s | %s\r\n", "index", "node", "role", "status")
	res += fmt.Sprintf("------+---------------------------------------------+----------+---------------\r\n")
	for i, member := range data {
		role := "follower"
		if member.IsLeader {
			role = "leader"
		}
		res += fmt.Sprintf("%-6d| %-43s | %-8s | %s\r\n", i, fmt.Sprintf("%s(%s:%d)", "", member.ServiceIP, member.Port), role, member.Status)
	}
	res += fmt.Sprintf("------+---------------------------------------------+----------+---------------\r\n")
	return res
}
