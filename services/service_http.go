package services

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/g"
)

// 创建一个新的http服务
func NewHttpService(ctx *g.Context) *HttpService {
	log.Debugf("start http service with config: %+v", ctx.Config.HTTP)
	if !ctx.Config.HTTP.Enabled {
		return &HttpService{
			status: 0,
		}
	}
	gc := len(ctx.Config.HTTP.Groups)
	client := &HttpService{
		lock:     new(sync.Mutex),
		groups:   make(httpGroups, gc),
		status:   serviceEnable,
		timeTick: time.Duration(ctx.Config.HTTP.TimeTick) * time.Second,
		ctx:      ctx,
	}
	for _, groupConfig := range ctx.Config.HTTP.Groups {
		httpGroup := newHttpGroup(ctx, groupConfig)
		client.lock.Lock()
		client.groups.add(httpGroup)
		client.lock.Unlock()
	}

	return client
}

// 开始服务
func (client *HttpService) Start() {
	if client.status&serviceEnable <= 0 {
		return
	}
	client.groups.sendService()
}

func (client *HttpService) SendAll(table string, data []byte) bool {
	if client.status&serviceEnable <= 0 {
		return false
	}
	client.groups.asyncSend(table, data)
	return true
}

func (client *HttpService) Close() {
	log.Debug("http service closing, waiting for buffer send complete.")
	client.groups.wait()
	log.Debug("http service closed.")
}

func (client *HttpService) Reload() {
	client.ctx.Reload()
	log.Debug("http service reloading...")

	client.status = 0
	if client.ctx.Config.HTTP.Enabled {
		client.status = serviceEnable
	}

	for _, group := range client.groups {
		client.groups.delete(group)
	}

	for _, groupConfig := range client.ctx.Config.HTTP.Groups {
		httpGroup := newHttpGroup(client.ctx, groupConfig)
		client.lock.Lock()
		client.groups.add(httpGroup)
		client.lock.Unlock()
	}
	log.Debug("http service reloaded.")
}

func (client *HttpService) Name() string {
	return "http"
}
