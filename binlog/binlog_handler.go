package binlog

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/siddontang/go-mysql/canal"
	"github.com/siddontang/go-mysql/mysql"
	"github.com/siddontang/go-mysql/replication"
	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/nova/app"
	"github.com/mia0x75/nova/path"
	"github.com/mia0x75/nova/services"
)

func (h *Binlog) handlerInit() {
	var err error
	mysqlBinlogCacheFile := app.MASTER_INFO_FILE
	path.Mkdir(path.GetParent(mysqlBinlogCacheFile))
	flag := os.O_RDWR | os.O_CREATE | os.O_SYNC
	h.cacheHandler, err = os.OpenFile(mysqlBinlogCacheFile, flag, 0755)
	if err != nil {
		log.Panicf("open cache file with error：%s, %+v", mysqlBinlogCacheFile, err)
	}
	h.statusLock.Lock()
	h.status |= _cacheHandlerIsOpened
	h.statusLock.Unlock()
	f, p, index := h.getBinlogPositionCache()
	atomic.StoreInt64(&h.EventIndex, index)
	h.setHandler()
	currentPos, err := h.handler.GetMasterPos()
	if err != nil {
		log.Panicf("get master pos with error：%+v", err)
	}
	log.Debugf("master pos: %+v", currentPos)
	if f != "" && p > 0 {
		h.Config.BinFile = f
		h.Config.BinPos = uint32(p)
		if f == currentPos.Name && h.Config.BinPos > currentPos.Pos {
			//pos set error, auto start form current pos
			h.Config.BinPos = currentPos.Pos
			log.Warnf("pos set error, auto start form: %d", h.Config.BinPos)
		}
	} else {
		h.Config.BinFile = currentPos.Name
		h.Config.BinPos = currentPos.Pos
	}
	h.lastBinFile = h.Config.BinFile
	h.lastPos = uint32(h.Config.BinPos)
	h.posChan = make(chan []byte, posChanLen)
	log.Debugf("current pos: (%+v, %+v)", h.lastBinFile, h.lastPos)
	go h.asyncSavePosition()
	go h.lookPosChange()
}

func (h *Binlog) asyncSavePosition() {
	h.wg.Add(1)
	defer h.wg.Done()
	for {
		select {
		case r, ok := <-h.posChan:
			if !ok {
				return
			}
			log.Debugf("write binlog pos cache: %+v", r)
			h.statusLock.Lock()
			if h.status&_cacheHandlerIsOpened > 0 {
				h.statusLock.Unlock()
				n, err := h.cacheHandler.WriteAt(r, 0)
				if err != nil || n <= 0 {
					log.Errorf("write binlog cache file with error: %+v", err)
				}
			} else {
				h.statusLock.Unlock()
				log.Warnf("handler is closed")
			}
		case <-h.ctx.Ctx.Done():
			if len(h.posChan) <= 0 {
				return
			}
		}
	}
}

func (h *Binlog) setHandler() {
	cfg, err := canal.NewConfigWithFile(app.DB_CONFIG_FILE)
	if err != nil {
		log.Panicf("new canal config with error：%+v", err)
	}
	handler, err := canal.NewCanal(cfg)
	if err != nil {
		log.Panicf("new canal with error：%+v", err)
	}
	h.lock.Lock()
	h.handler = handler
	h.lock.Unlock()
	h.handler.SetEventHandler(h)
}

func (h *Binlog) RegisterService(name string, s services.Service) {
	h.lock.Lock()
	h.services[name] = s
	h.lock.Unlock()
}

func (h *Binlog) notify(table string, data map[string]interface{}) {
	log.Debugf("binlog notify: %+v", data)
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Errorf("json pack data error[%v]: %v", err, data)
		return
	}
	for _, service := range h.services {
		service.SendAll(table, jsonData)
	}
}

func (h *Binlog) OnRow(e *canal.RowsEvent) error {
	h.statusLock.Lock()
	if h.status&_binlogIsExit > 0 {
		h.statusLock.Unlock()
		return nil
	}
	h.statusLock.Unlock()
	// 发生变化的数据表e.Table，如xsl.x_reports
	// 发生的操作类型e.Action，如update、insert、delete
	// 如update的数据，update的数据以双数出现前面为更新前的数据，后面的为更新后的数据
	// 0，2，4偶数的为更新前的数据，奇数的为更新后的数据
	// [[1 1 3074961 [115 102 103 98 114]   1 1485739538 1485739538]
	// [1 1 3074961 [115 102 103 98 114] 1 1 1485739538 1485739538]]
	// delete一次返回一条数据
	// delete的数据delete [[3 1 3074961 [97 115 100 99 97 100 115] 1,2,2 1 1485768268 1485768268]]
	// 一次插入多条的时候，同时返回
	// insert的数据insert xsl.x_reports [[6 0 0 [] 0 1 0 0]]
	rowData := make(map[string]interface{})
	rowData["database"] = e.Table.Schema
	rowData["event_type"] = e.Action
	rowData["time"] = time.Now().Unix()
	rowData["table"] = e.Table.Name

	data := make(map[string]interface{})
	ed := make(map[string]interface{})

	if e.Action == "update" {
		for i := 0; i < len(e.Rows); i += 2 {
			rowData["event_index"] = atomic.AddInt64(&h.EventIndex, int64(1))
			oldData := make(map[string]interface{})
			newData := make(map[string]interface{})
			rowsLen := len(e.Rows[i])
			for k, col := range e.Table.Columns {
				if k < rowsLen {
					oldData[col.Name] = fieldDecode(e.Rows[i][k], &col)
				} else {
					log.Warn("unknown line", col.Name)
					oldData[col.Name] = nil
				}
			}
			rowsLen = len(e.Rows[i+1])
			for k, col := range e.Table.Columns {
				if k < rowsLen {
					newData[col.Name] = fieldDecode(e.Rows[i+1][k], &col)
				} else {
					log.Warn("unknown line", col.Name)
					newData[col.Name] = nil
				}
			}
			data["old_data"] = oldData
			data["new_data"] = newData
			ed["data"] = data
			rowData["event"] = ed
			h.notify(e.Table.Name, rowData)
		}
	} else {
		for i := 0; i < len(e.Rows); i += 1 {
			rowData["event_index"] = atomic.AddInt64(&h.EventIndex, int64(1))
			rowsLen := len(e.Rows[i])
			for k, col := range e.Table.Columns {
				if k < rowsLen {
					data[col.Name] = fieldDecode(e.Rows[i][k], &col)
				} else {
					log.Warn("unknown line", col.Name)
					data[col.Name] = nil
				}
			}
			ed["data"] = data
			rowData["event"] = ed
			h.notify(e.Table.Name, rowData)
		}
	}
	return nil
}

func (h *Binlog) String() string {
	return "Binlog"
}

func (h *Binlog) OnRotate(e *replication.RotateEvent) error {
	log.Debugf("OnRotate event fired, %+v", e)
	return nil
}

func (h *Binlog) OnDDL(p mysql.Position, e *canal.DDLEvent) error {
	log.Infof("schema change detected, db: %s, table: %s, action: %s.", e.Schema, e.Table, e.Action)
	query := make(map[string]interface{})
	query["query"] = e.Query

	event := make(map[string]interface{})
	event["data"] = query

	data := make(map[string]interface{})
	data["database"] = e.Schema
	data["event_type"] = e.Action
	data["time"] = time.Now().Unix()
	data["table"] = e.Table
	data["event_index"] = atomic.AddInt64(&h.EventIndex, int64(1))
	data["event"] = event
	h.notify(e.Table, data)
	return nil
}

func (h *Binlog) OnXID(p mysql.Position) error {
	log.Debugf("OnXID event fired, %+v.", p)
	return nil
}

func (h *Binlog) OnGTID(g mysql.GTIDSet) error {
	log.Debugf("OnGTID event fired, GTID: %+v", g)
	return nil
}

func (h *Binlog) lookPosChange() {
	for {
		select {
		case data, ok := <-h.ctx.PosChan:
			if !ok {
				return
			}
			for {
				if data == "" || len(data) < 19 {
					log.Errorf("pos data error: %v", data)
					break
				}
				log.Debugf("onPosChange")
				file, pos, index := unpackPos([]byte(data))
				if file == "" || pos <= 0 {
					log.Errorf("error with: %s, %d", file, pos)
					break
				}
				h.lastBinFile = file
				h.lastPos = uint32(pos)
				atomic.StoreInt64(&h.EventIndex, index)
				r := packPos(file, pos, index)
				h.SaveBinlogPositionCache(r)
				break
			}
		case <-h.ctx.Ctx.Done():
			if len(h.ctx.PosChan) <= 0 {
				return
			}
		}
	}
}

func (h *Binlog) OnPosSynced(p mysql.Position, b bool) error {
	log.Debugf("OnPosSynced fired with data: %+v, %v", p, b)
	eventIndex := atomic.LoadInt64(&h.EventIndex)
	pos := int64(p.Pos)
	data := packPos(p.Name, pos, eventIndex)
	h.SaveBinlogPositionCache(data)
	for _, service := range h.services {
		service.SendPos(data)
	}
	h.lastBinFile = p.Name
	h.lastPos = p.Pos
	return nil
}

func (h *Binlog) SaveBinlogPositionCache(r []byte) {
	h.statusLock.Lock()
	if h.status&_binlogIsExit > 0 {
		h.statusLock.Unlock()
		return
	}
	h.statusLock.Unlock()
	for {
		if len(h.posChan) < cap(h.posChan) {
			break
		}
		log.Warnf("cache full, try wait")
	}
	h.posChan <- r
}

func (h *Binlog) getBinlogPositionCache() (string, int64, int64) {
	h.statusLock.Lock()
	if h.status&_cacheHandlerIsOpened <= 0 {
		h.statusLock.Unlock()
		log.Warnf("handler is closed")
		return "", 0, 0
	}
	h.statusLock.Unlock()
	h.cacheHandler.Seek(0, io.SeekStart)
	data := make([]byte, bytes.MinRead)
	n, err := h.cacheHandler.Read(data)
	if n <= 0 || err != nil {
		if err != io.EOF {
			log.Errorf("read pos error: %v", err)
		}
		return "", int64(0), int64(0)
	}
	return unpackPos(data)
}
