package services

import (
	"github.com/BurntSushi/toml"
	"github.com/mia0x75/centineld/file"
	log "github.com/sirupsen/logrus"
	"errors"
	"sync"
	"net"
	"context"
)

// 标准服务接口
type Service interface {
	// 发送消息
	SendAll(msg []byte) bool
	// 开始服务
	Start()
	Close()
	SetContext(ctx *context.Context)
	Reload()
}

type tcpGroupConfig struct {
	Name string  // = "group1"
	Filter []string
}

type tcpConfig struct {
	Listen string
	Port int
}

type TcpConfig struct {
	Enable bool
	Groups map[string]tcpGroupConfig
	Tcp tcpConfig
}

type HttpConfig struct {
	Enable bool
	TimeTick int //故障检测的时间间隔，单位为秒
	Groups map[string]httpNodeConfig
}

type httpNodeConfig struct {
	Nodes [][]string
	Filter []string
}

type tcpClientNode struct {
	conn *net.Conn           // 客户端连接进来的资源句柄
	is_connected bool        // 是否还连接着 true 表示正常 false表示已断开
	send_queue chan []byte   // 发送channel
	send_failure_times int64 // 发送失败次数
	group string             // 所属分组
	recv_buf []byte          // 读缓冲区
	recv_bytes int           // 收到的待处理字节数量
	connect_time int64       // 连接成功的时间戳
	send_times int64         // 发送次数，用来计算负载均衡，如果 mode == 2
}

type TcpService struct {
	Service
	Ip string                             // 监听ip
	Port int                              // 监听端口
	recv_times int64                      // 收到消息的次数
	send_times int64                      // 发送消息的次数
	send_failure_times int64              // 发送失败的次数
	lock *sync.Mutex                      // 互斥锁，修改资源时锁定
	groups map[string][]*tcpClientNode    // 客户端分组，现在支持两种分组，广播组合负载均衡组
	groups_filter map[string] []string    // 分组的过滤器
	clients_count int32                   // 成功连接（已经进入分组）的客户端数量
	enable bool
	ctx *context.Context
	listener *net.Listener
}

type HttpService struct {
	Service
	//send_queue chan []byte     // 发送channel
	groups [][]*httpNode       // 客户端分组，现在支持两种分组，广播组合负载均衡组
	groups_filter [][]string   // 分组过滤器
	lock *sync.Mutex           // 互斥锁，修改资源时锁定
	send_failure_times int64   // 发送失败次数
	enable bool
	time_tick int              // 故障检测的时间间隔
	ctx *context.Context
}

type httpNode struct {
	url string                  // url
	send_queue chan string      // 发送channel
	send_times int64            // 发送次数
	send_failure_times int64    // 发送失败次数
	is_down bool                // 是否因为故障下线的节点
	failure_times_flag int32    // 发送失败次数，用于配合last_error_time检测故障，故障定义为：连续三次发生错误和返回错误
	lock *sync.Mutex            // 互斥锁，修改资源时锁定
	cache [][]byte
	cache_index int
	cache_is_init bool
	cache_full bool
}

var (
	ErrorFileNotFound = errors.New("配置文件不存在")
	ErrorFileParse = errors.New("配置解析错误")
)

const (
	CMD_SET_PRO = 1 // 注册客户端操作，加入到指定分组
	CMD_AUTH    = 2 // 认证（暂未使用）
	CMD_OK      = 3 // 正常响应
	CMD_ERROR   = 4 // 错误响应
	CMD_TICK    = 5 // 心跳包
	CMD_EVENT   = 6 // 事件

	TCP_MAX_SEND_QUEUE            = 1000000 //100万缓冲区
	TCP_DEFAULT_CLIENT_SIZE       = 64
	TCP_DEFAULT_READ_BUFFER_SIZE  = 1024
	TCP_RECV_DEFAULT_SIZE         = 4096
	TCP_DEFAULT_WRITE_BUFFER_SIZE = 4096

	HTTP_CACHE_LEN         = 10000
	HTTP_CACHE_BUFFER_SIZE = 4096
)

func getTcpConfig() (*TcpConfig, error) {
	var tcp_config TcpConfig
	config_file := "/etc/centineld/tcp.toml"
	wfile := file.WFile{config_file}
	if !wfile.Exists() {
		log.Warnf("配置文件%s不存在 %s", config_file)
		return nil, ErrorFileNotFound
	}
	if _, err := toml.DecodeFile(config_file, &tcp_config); err != nil {
		log.Println(err)
		return nil, ErrorFileParse
	}
	return &tcp_config, nil
}

func getHttpConfig() (*HttpConfig, error) {
	var config HttpConfig
	config_file := "/etc/centineld/http.toml"
	wfile := file.WFile{config_file}
	if !wfile.Exists() {
		log.Warnf("配置文件%s不存在 %s", config_file)
		return nil, ErrorFileNotFound
	}
	if _, err := toml.DecodeFile(config_file, &config); err != nil {
		log.Println(err)
		return nil, ErrorFileParse
	}
	if config.TimeTick <= 0 {
		config.TimeTick = 1
	}
	return &config, nil
}
