package binlog

import (
	"github.com/siddontang/go-mysql/canal"
	"github.com/siddontang/go-mysql/mysql"
	"os"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/mia0x75/centineld/file"
	"github.com/mia0x75/centineld/services"
	"context"
)

func NewBinlog() *Binlog {
	config, _ := GetMysqlConfig()
	binlog := &Binlog {
		Config:config,
	}
	
	cfg := canal.NewDefaultConfig()
	cfg.Addr = fmt.Sprintf("%s:%d", config.Host, config.Port)
	cfg.User = config.User
	cfg.Password = config.Password
	cfg.Flavor = config.Flavor

	cfg.ReadTimeout = config.ReadTimeout
	cfg.HeartbeatPeriod = config.HeartbeatPeriod
	cfg.ServerID = config.ServerID
	cfg.Charset = config.Charset
	
	var err error
	binlog.handler, err = canal.NewCanal(cfg)
	if err != nil {
		log.Panicf("Binlog::NewBinlog - binlog创建canal错误：%+v", err)
		os.Exit(1)
	}
	f, p, index := binlog.BinlogHandler.getBinlogPositionCache()
	var b [defaultBufSize]byte
	binlog.BinlogHandler = binlogHandler{
		Event_index: index,
		services:make(map[string]services.Service),
		services_count:0,
	}
	binlog.BinlogHandler.buf = b[:0]
	binlog.handler.SetEventHandler(&binlog.BinlogHandler)
	binlog.is_connected = false
	current_pos, err:= binlog.handler.GetMasterPos()
	if f != "" {
		binlog.Config.BinFile = f
	} else {
		if err != nil {
			log.Panicf("binlog获取GetMasterPos错误：%+v", err)
		} else {
			binlog.Config.BinFile = current_pos.Name
		}
	}
	if p > 0 {
		binlog.Config.BinPos = p
	} else {
		if err != nil {
			log.Panicf("binlog获取GetMasterPos错误：%+v", err)
		} else {
			binlog.Config.BinPos = int64(current_pos.Pos)
		}
	}
	log.Debugf("Binlog::NewBinlog - %+v", binlog.Config)

	// 初始化缓存文件句柄
	mysql_binlog_position_cache := "/run/centineld/centineld.cache"
	dir := file.WPath{mysql_binlog_position_cache}
	dir = file.WPath{dir.GetParent()}
	dir.Mkdir()
	flag := os.O_WRONLY | os.O_CREATE | os.O_SYNC | os.O_TRUNC
	binlog.BinlogHandler.cacheHandler, err = os.OpenFile(
		mysql_binlog_position_cache, flag , 0755)
	if err != nil {
		log.Panicf("binlog服务，打开缓存文件错误：%s, %+v", mysql_binlog_position_cache, err)
	}
	return binlog
}

func (h *Binlog) Close() {
	log.Debug("binlog服务退出...")
	if !h.is_connected  {
		return
	}
	h.is_connected = false
	for _, service := range h.BinlogHandler.services {
		log.Debug("服务退出...")
		service.Close()
	}
	log.Debug("binlog-服务Close-all退出...")
	h.BinlogHandler.cacheHandler.Close()
	log.Debug("binlog-h.BinlogHandler.cacheHandler.Close退出...")
	h.handler.Close()
	log.Debug("binlog-h.handler.Close退出...")
}


func (h *Binlog) Start(ctx *context.Context) {
	h.ctx = ctx
	for _, service := range h.BinlogHandler.services {
		service.Start()
		service.SetContext(ctx)
	}
	log.Debugf("Binlog::Start - binlog调试：%s,%d", h.Config.BinFile, uint32(h.Config.BinPos))
	go func() {
		startPos := mysql.Position{
			Name: h.Config.BinFile,
			Pos:  uint32(h.Config.BinPos),
		}
		h.is_connected = true
		err := h.handler.RunFrom(startPos)
		if err != nil {
			log.Fatalf("Binlog::Start - binlog服务：start canal err %v", err)
			return
		}
	}()
}

func (h *Binlog) Reload(service string) {
	var (
		tcp = "tcp"
		http = "http"
		all = "all"
	)
	switch service {
	case tcp:
		log.Debugf("重新加载tcp服务")
		h.BinlogHandler.services["tcp"].Reload()
	case http:
		log.Debugf("重新加载http服务")
		h.BinlogHandler.services["http"].Reload()
	case all:
		log.Debugf("重新加载全部服务")
		h.BinlogHandler.services["tcp"].Reload()
		h.BinlogHandler.services["http"].Reload()
	}
}
