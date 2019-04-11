package binlog

import (
	"os"
	"sync"

	"github.com/siddontang/go-mysql/canal"

	"github.com/mia0x75/copycat/g"
	"github.com/mia0x75/copycat/services"
)

// Binlog TODO
type Binlog struct {
	canal.DummyEventHandler                              // github.com/siddontang/go-mysql interface
	handler                 *canal.Canal                 // github.com/siddontang/go-mysql mysql protocol handler
	ctx                     *g.Context                   // context, like that use for wait coroutine exit
	wg                      *sync.WaitGroup              // use for wait coroutine exit
	lock                    *sync.Mutex                  // lock
	statusLock              *sync.Mutex                  // status lock
	EventIndex              int64                        // event unique index
	services                map[string]services.IService // registered service, key is the name of the service
	cacheHandler            *os.File                     // cache handler, binlog_handler.go SaveBinlogPostionCache and getBinlogPositionCache
	lastPos                 uint32                       // the last read pos
	lastBinFile             string                       // the last read binlog file
	startServiceChan        chan struct{}                //
	stopServiceChan         chan bool                    //
	status                  int                          // binlog status

	//pos change 回调函数
	onPosChanges []PosChangeFunc
	onEvent      []OnEventFunc
}

// Option TODO
type Option func(h *Binlog)

// PosChangeFunc TODO
type PosChangeFunc func(r []byte)

// OnEventFunc TODO
type OnEventFunc func(table string, data []byte)

const (
	binlogIsRunning = 1 << iota
	binlogIsExit
	cacheHandlerIsOpened
)
