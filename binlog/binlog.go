package binlog

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/siddontang/go-mysql/mysql"
	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/nova/app"
	"github.com/mia0x75/nova/services"
)

func NewBinlog(ctx *app.Context, opts ...BinlogOption) *Binlog {
	binlog := &Binlog{
		Config:           ctx.MysqlConfig,                   //
		wg:               new(sync.WaitGroup),               //
		lock:             new(sync.Mutex),                   //
		statusLock:       new(sync.Mutex),                   //
		ctx:              ctx,                               //
		services:         make(map[string]services.Service), //
		startServiceChan: make(chan struct{}, 100),          //
		stopServiceChan:  make(chan bool, 100),              //
		status:           0,                                 //
		onPosChanges:     make([]PosChangeFunc, 0),          //
	}
	if len(opts) > 0 {
		for _, f := range opts {
			f(binlog)
		}
	}
	binlog.handlerInit()
	binlog.lookService()
	return binlog
}

// if pos change, will call h.onPosChanges func
func PosChange(f PosChangeFunc) BinlogOption {
	return func(h *Binlog) {
		h.onPosChanges = append(h.onPosChanges, f)
	}
}

// set on event callback
func OnEvent(f OnEventFunc) BinlogOption {
	return func(h *Binlog) {
		h.onEvent = append(h.onEvent, f)
	}
}

func (h *Binlog) Close() {
	h.statusLock.Lock()
	if h.status&_binlogIsExit > 0 {
		h.statusLock.Unlock()
		return
	}
	h.status |= _binlogIsExit
	h.statusLock.Unlock()
	log.Warn("binlog service exit")
	h.StopService(true)
	for name, service := range h.services {
		log.Debugf("%s service exit", name)
		service.Close()
	}
	h.wg.Wait()
}

// for start and stop binlog service
func (h *Binlog) lookService() {
	h.wg.Add(2)
	go func() {
		defer h.wg.Done()
		for {
			select {
			case _, ok := <-h.startServiceChan:
				if !ok {
					return
				}
				for {
					h.statusLock.Lock()
					if h.status&_binlogIsRunning > 0 {
						h.statusLock.Unlock()
						break
					}
					h.status |= _binlogIsRunning
					h.statusLock.Unlock()
					log.Debug("binlog service start")
					go func() {
						for {
							if h.lastBinFile == "" {
								log.Warn("binlog lastBinFile is empty, wait for init")
								time.Sleep(time.Second)
								continue
							}
							break
						}
						startPos := mysql.Position{
							Name: h.lastBinFile,
							Pos:  h.lastPos,
						}
						for {
							if h.handler == nil {
								log.Warn("binlog handler is nil, wait for init")
								time.Sleep(time.Second)
								continue
							}
							break
						}
						err := h.handler.RunFrom(startPos)
						if err != nil {
							log.Warnf("binlog service exit with error: %+v", err)
							h.statusLock.Lock()
							h.status ^= _binlogIsRunning
							h.statusLock.Unlock()
							return
						}
					}()
					break
				}
			case <-h.ctx.Ctx.Done():
				return
			}
		}
	}()
	go func() {
		defer h.wg.Done()
		for {
			select {
			case exit, ok := <-h.stopServiceChan:
				if !ok {
					return
				}
				h.statusLock.Lock()
				if h.status&_binlogIsRunning > 0 && !exit {
					h.statusLock.Unlock()
					log.Debug("binlog service stop")
					h.handler.Close()
					h.setHandler()
				} else {
					h.statusLock.Unlock()
				}
				if exit {
					r := packPos(h.lastBinFile, int64(h.lastPos), atomic.LoadInt64(&h.EventIndex))
					h.saveBinlogPositionCache(r)
					h.statusLock.Lock()
					if h.status&_cacheHandlerIsOpened > 0 {
						h.status ^= _cacheHandlerIsOpened
						h.statusLock.Unlock()
						h.cacheHandler.Close()
					} else {
						h.statusLock.Unlock()
					}
				}
				h.statusLock.Lock()
				if h.status&_binlogIsRunning > 0 {
					h.status ^= _binlogIsRunning
				}
				h.statusLock.Unlock()
			case <-h.ctx.Ctx.Done():
				return
			}
		}
	}()
}

func (h *Binlog) StopService(exit bool) {
	log.Debugf("===========binlog service stop was called===========")
	h.stopServiceChan <- exit
}

func (h *Binlog) StartService() {
	log.Debugf("===========binlog service start was called===========")
	h.startServiceChan <- struct{}{}
}

func (h *Binlog) Start() {
	for _, service := range h.services {
		service.Start()
	}
}

func (h *Binlog) OnLeader(isLeader bool) {
	log.Debugf("========OnLeader call========")
	if isLeader {
		// leader start service
		h.StartService()
	} else {
		// if not leader, stop service
		h.StopService(false)
	}
}
