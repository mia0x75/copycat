package binlog

import (
	"errors"
	"os"
	"sync"

	"github.com/hashicorp/consul/api"
	"github.com/siddontang/go-mysql/canal"

	"github.com/mia0x75/nova/app"
	"github.com/mia0x75/nova/services"
)

var (
	sessionEmpty = errors.New("session empty")
	pingData     = services.PackPro(services.FlagPing, []byte("ping"))
)

type Binlog struct {
	canal.DummyEventHandler                             // github.com/siddontang/go-mysql interface
	Config                  *app.MysqlConfig            // config
	handler                 *canal.Canal                // github.com/siddontang/go-mysql mysql protocol handler
	ctx                     *app.Context                // context, like that use for wait coroutine exit
	wg                      *sync.WaitGroup             // use for wait coroutine exit
	lock                    *sync.Mutex                 // lock
	statusLock              *sync.Mutex                 // status lock
	EventIndex              int64                       // event unique index
	services                map[string]services.Service // registered service, key is the name of the service
	cacheHandler            *os.File                    // cache handler, binlog_handler.go SaveBinlogPostionCache and getBinlogPositionCache
	lastPos                 uint32                      // the last read pos
	lastBinFile             string                      // the last read binlog file
	LockKey                 string                      // consul lock key
	Address                 string                      // consul address
	ServiceIp               string                      // tcp service ip
	ServicePort             int                         // tcp service port
	Session                 *Session                    // consul session client
	sessionId               string                      // unique session id
	Client                  *api.Client                 // consul client api
	Kv                      *api.KV                     // consul kv service
	agent                   *api.Agent                  // consul agent, use for register service
	startServiceChan        chan struct{}               //
	stopServiceChan         chan bool                   //
	posChan                 chan []byte                 //
	status                  int                         // binlog status
}

const (
	_binlogIsRunning = 1 << iota
	_binlogIsExit
	_cacheHandlerIsOpened
	_consulIsLeader
	_enableConsul
)

const (
	serviceKeepaliveTimeout = 6 // timeout, unit is second
	checkAliveInterval      = 1 // interval for checkalive
	keepaliveInterval       = 1 // interval for keepalive

	prefixKeepalive = "nova/binlog/keepalive/"
	statusOnline    = "online"
	statusOffline   = "offline"
	ServiceNameTcp  = "tcp"
	ServiceNameHttp = "http"
	posChanLen      = 10000
)

// cluster interface
type Cluster interface {
	Close()
	Lock() bool
	Write(data []byte) bool
	GetMembers() []*ClusterMember
	ClearOfflineMembers()
	GetServices() map[string]*api.AgentService
	GetLeader() (string, int)
}

// cluster node(member)
type ClusterMember struct {
	Hostname  string
	IsLeader  bool
	SessionId string
	Status    string
	ServiceIp string
	Port      int
}
