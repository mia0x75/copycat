package binlog

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/siddontang/go-mysql/mysql"
	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/g"
	"github.com/mia0x75/copycat/services"
)

// NewBinlog 创建一个binlog服务对象
func NewBinlog(ctx *g.Context, opts ...Option) *Binlog {
	binlog := &Binlog{
		Config:           ctx.Config.Database,               //
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
	for _, f := range opts {
		f(binlog)
	}
	binlog.handlerInit()
	go binlog.lookStartService()
	go binlog.lookStopService()
	return binlog
}

// PosChange set pos change callback
// if pos change, will call h.onPosChanges func
// 设置binlog pos改变回调api
func PosChange(f PosChangeFunc) Option {
	return func(h *Binlog) {
		h.onPosChanges = append(h.onPosChanges, f)
	}
}

// OnEvent set on event callback
// 设置事件回调api
func OnEvent(f OnEventFunc) Option {
	return func(h *Binlog) {
		h.onEvent = append(h.onEvent, f)
	}
}

// Close 关闭binlog服务
func (h *Binlog) Close() {
	h.statusLock.Lock()
	if h.status&binlogIsExit > 0 {
		h.statusLock.Unlock()
		return
	}
	h.status |= binlogIsExit
	h.statusLock.Unlock()
	log.Warn("binlog service exit")
	h.StopService(true)
	for _, service := range h.services {
		service.Close()
	}
	h.wg.Wait()
}

// for start and stop binlog service
// 监听启动服务信号
// 选leader时，如果成功被选为leader，则会发出启动binlog服务启动信号，由lookStartService处理
// 非leader则会收到停止服务信号，由lookStopService停止binlog服务
func (h *Binlog) lookStartService() {
	h.wg.Add(1)
	defer h.wg.Done()
	for {
		select {
		case _, ok := <-h.startServiceChan:
			if !ok {
				return
			}
			for {
				h.statusLock.Lock()
				if h.status&binlogIsRunning > 0 {
					h.statusLock.Unlock()
					break
				}
				h.status |= binlogIsRunning
				h.statusLock.Unlock()
				log.Debug("[D] binlog service start")
				go func() {
					start := time.Now().Unix()
					for {
						if h.lastBinFile == "" {
							log.Warn("[W] binlog lastBinFile is empty, wait for init")
							if time.Now().Unix()-start > 3 {
								log.Panicf("[P] binlog last file is empty")
							}
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
						log.Warnf("[W] binlog service exit with error: %+v", err)
						h.statusLock.Lock()
						h.status ^= binlogIsRunning
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
}

// 监听停止服务信号
// 选leader时，如果成功被选为leader，则会发出启动binlog服务启动信号，由lookStartService处理
// 非leader则会收到停止服务信号，由lookStopService停止binlog服务
func (h *Binlog) lookStopService() {
	h.wg.Add(1)
	defer h.wg.Done()
	for {
		select {
		case exit, ok := <-h.stopServiceChan:
			if !ok {
				return
			}
			h.statusLock.Lock()
			if h.status&binlogIsRunning > 0 && !exit {
				log.Debug("[D] binlog service stop")
				h.handler.Close()
				//reset handler
				h.setHandler()
			}
			h.statusLock.Unlock()

			if exit {
				h.statusLock.Lock()
				r := packPos(h.lastBinFile, int64(h.lastPos), atomic.LoadInt64(&h.EventIndex))
				h.statusLock.Unlock()
				h.saveBinlogPositionCache(r)
				h.statusLock.Lock()
				if h.status&cacheHandlerIsOpened > 0 {
					h.status ^= cacheHandlerIsOpened
					h.statusLock.Unlock()
					h.cacheHandler.Close()
				} else {
					h.statusLock.Unlock()
				}
			}
			h.statusLock.Lock()
			if h.status&binlogIsRunning > 0 {
				h.status ^= binlogIsRunning
			}
			h.statusLock.Unlock()
		case <-h.ctx.Ctx.Done():
			return
		}
	}
}

// StopService 停止服务
// 参数exit为true时，会彻底退出服务
// 这里只是发出了停止服务信号
func (h *Binlog) StopService(exit bool) {
	log.Debugf("[D] ===========binlog service stop was called===========")
	h.stopServiceChan <- exit
}

// StartService 启动服务
// 这里只是发出了启动服务信号
func (h *Binlog) StartService() {
	log.Debugf("[D] ===========binlog service start was called===========")
	h.startServiceChan <- struct{}{}
}

// Start 启动binlog
// 这里启动的是服务插件
func (h *Binlog) Start() {
	for _, service := range h.services {
		log.Debugf("[D] try start service: %v", service.Name())
		service.Start()
	}
}

// OnLeader 选leader回调
// binlog服务启动和停止由此控制
func (h *Binlog) OnLeader(isLeader bool) {
	if isLeader {
		// leader start service
		h.StartService()
	} else {
		// if not leader, stop service
		h.StopService(false)
	}
}
