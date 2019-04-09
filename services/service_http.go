package services

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/g"
)

// NewHTTPService 创建一个新的http服务
func NewHTTPService(ctx *g.Context) *HTTPService {
	log.Debugf("[D] start http service with config: %+v", ctx.Config.HTTP)
	if !ctx.Config.HTTP.Enabled {
		return &HTTPService{
			status: 0,
		}
	}
	gc := len(ctx.Config.HTTP.Groups)
	client := &HTTPService{
		lock:     new(sync.Mutex),
		groups:   make(httpGroups, gc),
		status:   serviceEnable,
		timeTick: time.Duration(ctx.Config.HTTP.TimeTick) * time.Second,
		ctx:      ctx,
	}
	for _, groupConfig := range ctx.Config.HTTP.Groups {
		httpGroup := newHTTPGroup(ctx, groupConfig)
		client.lock.Lock()
		client.groups.add(httpGroup)
		client.lock.Unlock()
	}

	return client
}

// Start 开始服务
func (client *HTTPService) Start() {
	if client.status&serviceEnable <= 0 {
		return
	}
	client.groups.sendService()
}

// SendAll TODO
func (client *HTTPService) SendAll(table string, data []byte) bool {
	if client.status&serviceEnable <= 0 {
		return false
	}
	client.groups.asyncSend(table, data)
	return true
}

// Close TODO
func (client *HTTPService) Close() {
	log.Debug("[D] http service closing, waiting for buffer send complete.")
	client.groups.wait()
	log.Debug("[D] http service closed.")
}

// Reload TODO
func (client *HTTPService) Reload() {
	client.ctx.Reload()
	log.Debug("[D] http service reloading...")

	client.status = 0
	if client.ctx.Config.HTTP.Enabled {
		client.status = serviceEnable
	}

	for _, group := range client.groups {
		client.groups.delete(group)
	}

	for _, groupConfig := range client.ctx.Config.HTTP.Groups {
		httpGroup := newHTTPGroup(client.ctx, groupConfig)
		client.lock.Lock()
		client.groups.add(httpGroup)
		client.lock.Unlock()
	}
	log.Debug("[D] http service reloaded.")
}

// Name TODO
func (client *HTTPService) Name() string {
	return "http"
}
