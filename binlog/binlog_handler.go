package binlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/siddontang/go-mysql/canal"
	"github.com/siddontang/go-mysql/mysql"
	"github.com/siddontang/go-mysql/replication"
	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/g"
	"github.com/mia0x75/copycat/services"
	"github.com/mia0x75/copycat/utils/path"
)

// 初始化binlog事件相关句柄
func (h *Binlog) handlerInit() {
	var err error
	mysqlBinlogCacheFile := g.MASTER_INFO_FILE
	path.Mkdir(path.GetParent(mysqlBinlogCacheFile))
	flag := os.O_RDWR | os.O_CREATE | os.O_SYNC
	h.cacheHandler, err = os.OpenFile(mysqlBinlogCacheFile, flag, 0755)
	if err != nil {
		log.Panicf("[P] open cache file with error：%s, %+v", mysqlBinlogCacheFile, err)
	}
	h.statusLock.Lock()
	h.status |= cacheHandlerIsOpened
	h.statusLock.Unlock()
	f, p, index := h.getBinlogPositionCache()
	atomic.StoreInt64(&h.EventIndex, index)
	h.setHandler()
	currentPos, err := h.handler.GetMasterPos()
	if err != nil {
		log.Panicf("[P] get master pos with error：%+v", err)
	}
	log.Debugf("[D] master pos: %+v", currentPos)
	if f != "" && p > 0 {
		h.Config.BinlogFile = f
		h.Config.BinlogPos = uint32(p)
		if f == currentPos.Name && h.Config.BinlogPos > currentPos.Pos {
			//pos set error, auto start form current pos
			h.Config.BinlogPos = currentPos.Pos
			log.Warnf("[W] pos set error, auto start form: %d", h.Config.BinlogPos)
		}
	} else {
		h.Config.BinlogFile = currentPos.Name
		h.Config.BinlogPos = currentPos.Pos
	}
	h.lastBinFile = h.Config.BinlogFile
	h.lastPos = uint32(h.Config.BinlogPos)
	log.Debugf("[D] current pos: (%+v, %+v)", h.lastBinFile, h.lastPos)
}

// 设置binlog句柄为当前实现类
func (h *Binlog) setHandler() {
	cfg := canal.NewDefaultConfig()
	cfg.Addr = fmt.Sprintf("%s:%d", g.Config().Database.Host, g.Config().Database.Port)
	cfg.User = g.Config().Database.User
	cfg.Password = g.Config().Database.Password
	cfg.Charset = g.Config().Database.Charset
	cfg.ServerID = g.Config().Database.ServerID
	cfg.Flavor = g.Config().Database.Flavor
	cfg.HeartbeatPeriod = time.Duration(g.Config().Database.HeartbeatPeriod)
	cfg.ReadTimeout = time.Duration(g.Config().Database.ReadTimeout)

	handler, err := canal.NewCanal(cfg)
	if err != nil {
		log.Panicf("[P] new canal with error：%+v", err)
	}
	h.lock.Lock()
	h.handler = handler
	h.lock.Unlock()
	h.handler.SetEventHandler(h)
}

// RegisterService 注册服务
func (h *Binlog) RegisterService(s services.Service) {
	h.lock.Lock()
	h.services[s.Name()] = s
	h.lock.Unlock()
}

// notify 事件广播通知
func (h *Binlog) notify(data map[string]interface{}) {
	log.Debugf("[D] binlog notify: %+v", data)
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Errorf("[E] json pack data error[%v]: %v", err, data)
		return
	}
	table := data["database"].(string) + "." + data["table"].(string)
	for _, service := range h.services {
		service.SendAll(table, jsonData)
	}

	for _, f := range h.onEvent {
		f(table, jsonData)
	}
}

// OnRow 数据改变事件回调
func (h *Binlog) OnRow(e *canal.RowsEvent) error {
	h.statusLock.Lock()
	if h.status&binlogIsExit > 0 {
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
			h.notify(rowData)
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
			h.notify(rowData)
		}
	}
	return nil
}

// String 基础接口
func (h *Binlog) String() string {
	return "Binlog"
}

// OnRotate 暂未使用的基础事件
func (h *Binlog) OnRotate(e *replication.RotateEvent) error {
	log.Debugf("[D] OnRotate event fired, %+v", e)
	return nil
}

// alter table结构改变事件回调
// func (h *Binlog) OnDDL(p mysql.Position, e *canal.DDLEvent) error {
// 	log.Infof("[I] schema change detected, db: %s, table: %s, action: %s.", e.Schema, e.Table, e.Action)
// 	query := make(map[string]interface{})
// 	query["query"] = e.Query

// 	event := make(map[string]interface{})
// 	event["data"] = query

// 	data := make(map[string]interface{})
// 	data["database"] = e.Schema
// 	data["event_type"] = e.Action
// 	data["time"] = time.Now().Unix()
// 	data["table"] = e.Table
// 	data["event_index"] = atomic.AddInt64(&h.EventIndex, int64(1))
// 	data["event"] = event
// 	h.notify(data)
// 	return nil
// }

// OnXID 暂未使用的基础事件
func (h *Binlog) OnXID(p mysql.Position) error {
	log.Debugf("[D] OnXID event fired, %+v.", p)
	return nil
}

// OnGTID 暂未使用的基础事件
func (h *Binlog) OnGTID(g mysql.GTIDSet) error {
	log.Debugf("[D] OnGTID event fired, GTID: %+v", g)
	return nil
}

// OnPosSynced 二进制日志位置改变事件
func (h *Binlog) OnPosSynced(p mysql.Position, b bool) error {
	log.Debugf("[D] OnPosSynced fired with data: %+v, %v", p, b)
	eventIndex := atomic.LoadInt64(&h.EventIndex)
	pos := int64(p.Pos)
	data := packPos(p.Name, pos, eventIndex)
	h.saveBinlogPositionCache(data)
	h.lastBinFile = p.Name
	h.lastPos = p.Pos
	return nil
}

// use for agent sync pos callback
// 保存pos信息到cache
// 这里的api对外提供，用于agent集群同步pos信息
func (h *Binlog) SaveBinlogPosition(r []byte) {
	file, pos, index := unpackPos(r)
	h.lastBinFile = file    //p.Name
	h.lastPos = uint32(pos) //p.Pos
	atomic.StoreInt64(&h.EventIndex, index)
	h.saveBinlogPositionCache(r)
}

// agent 接收到pos改变的时候也会回调到这里
// 保存pos信息到cache
func (h *Binlog) saveBinlogPositionCache(r []byte) {
	h.statusLock.Lock()
	if h.status&binlogIsExit > 0 {
		h.statusLock.Unlock()
		return
	}
	log.Debugf("[D] write binlog pos cache: %+v", r)
	h.statusLock.Unlock()
	if h.status&cacheHandlerIsOpened > 0 {
		n, err := h.cacheHandler.WriteAt(r, 0)
		if err != nil || n <= 0 {
			log.Errorf("[E] write binlog cache file with error: %+v", err)
		}
	} else {
		log.Warnf("[W] handler is closed")
	}
	//只有leader才发送
	for _, f := range h.onPosChanges {
		f(r)
	}
}

// 读取cache中的pos信息
// 返回值分别为binlog file，binlog pos，event index 事件索引
func (h *Binlog) getBinlogPositionCache() (string, int64, int64) {
	h.statusLock.Lock()
	if h.status&cacheHandlerIsOpened <= 0 {
		h.statusLock.Unlock()
		log.Warnf("[W] handler is closed")
		return "", 0, 0
	}
	h.statusLock.Unlock()
	h.cacheHandler.Seek(0, io.SeekStart)
	data := make([]byte, bytes.MinRead)
	n, err := h.cacheHandler.Read(data)
	if n <= 0 || err != nil {
		if err != io.EOF {
			log.Errorf("[E] read pos error: %v", err)
		}
		return "", int64(0), int64(0)
	}
	return unpackPos(data)
}
