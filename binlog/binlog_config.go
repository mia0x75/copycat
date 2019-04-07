package binlog

import (
	"os"
	"sync"

	"github.com/siddontang/go-mysql/canal"

	"github.com/mia0x75/nova/app"
	"github.com/mia0x75/nova/services"
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
	startServiceChan        chan struct{}               //
	stopServiceChan         chan bool                   //
	posChan                 chan []byte                 //
	status                  int                         // binlog status

	//pos change 回调函数
	onPosChanges []PosChangeFunc
	onEvent      []OnEventFunc
}

type BinlogOption func(h *Binlog)
type PosChangeFunc func(r []byte)
type OnEventFunc func(table string, data []byte)

const (
	_binlogIsRunning = 1 << iota
	_binlogIsExit
	_cacheHandlerIsOpened
)

const (
	posChanLen = 10000
)
