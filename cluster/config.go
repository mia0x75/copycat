package cluster

import (
	"net"
	"sync"
	"github.com/mia0x75/centineld/buffer"
	"github.com/mia0x75/centineld/file"
	"github.com/BurntSushi/toml"
	log "github.com/sirupsen/logrus"
	"errors"
)
var (
	ErrorFileNotFound = errors.New("config file not fount")
	ErrorFileParse = errors.New("parse config error")
)
const (
	TCP_MAX_SEND_QUEUE            = 1000000 //100万缓冲区
	TCP_DEFAULT_CLIENT_SIZE       = 64
	TCP_DEFAULT_READ_BUFFER_SIZE  = 1024
	TCP_RECV_DEFAULT_SIZE         = 4096
	TCP_DEFAULT_WRITE_BUFFER_SIZE = 4096
	CLUSTER_NODE_DEFAULT_SIZE     = 4
)

const (
	CMD_APPEND_NODE   = 1
	CMD_APPEND_NET    = 2
	CMD_CONNECT_FIRST = 3
	CMD_APPEND_NODE_SURE = 4
)

type Cluster struct {
	Listen string      //监听ip，一般为0.0.0.0即可
	Port int           //节点端口
	ServiceIp string   //对外服务ip
	is_down bool       //是否已下线
	client *tcp_client
	server *tcp_server
	nodes []*cluster_node
	nodes_count int
	lock *sync.Mutex
}

type cluster_node struct {
	service_ip string
	port int
	is_enable bool
}

type tcp_client struct {
	ip string
	port int
	conn *net.Conn
	is_closed bool
	recv_times int64
	recv_buf *buffer.WBuffer //[]byte
	client_id string         //用来标识一个客户端，随机字符串
	lock *sync.Mutex          // 互斥锁，修改资源时锁定
}

type tcp_client_node struct {
	conn *net.Conn           // 客户端连接进来的资源句柄
	is_connected bool        // 是否还连接着 true 表示正常 false表示已断开
	send_queue chan []byte   // 发送channel
	send_failure_times int64 // 发送失败次数
	weight int               // 权重 0 - 100
	recv_buf *buffer.WBuffer //[]byte          // 读缓冲区
	connect_time int64       // 连接成功的时间戳
	send_times int64         // 发送次数，用来计算负载均衡，如果 mode == 2
}

type tcp_server struct {
	listen string
	service_ip string
	cluster *Cluster
	port int
	client *tcp_client
	clients []*tcp_client_node
	lock *sync.Mutex          // 互斥锁，修改资源时锁定
	clients_count int
}

type cluster_config struct{
	Cluster node_config
}

type node_config struct {
	Listen string
	Port int
	ServiceIp string
}

func getServiceConfig() (*cluster_config, error) {
	var config cluster_config
	config_file := "/etc/centineld/cluster.toml"
	wfile := file.WFile{config_file}
	if !wfile.Exists() {
		log.Errorf("config file %s does not exists", config_file)
		return nil, ErrorFileNotFound
	}

	if _, err := toml.DecodeFile(config_file, &config); err != nil {
		log.Println(err)
		return nil, ErrorFileParse
	}
	return &config, nil
}
