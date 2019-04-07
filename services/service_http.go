package services

import (
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/nova/app"
)

// 创建一个新的http服务
func NewHttpService(ctx *app.Context) *HttpService {
	log.Debugf("start http service with config: %+v", ctx.HttpConfig)
	if !ctx.HttpConfig.Enable {
		return &HttpService{
			status: 0,
		}
	}
	gc := len(ctx.HttpConfig.Groups)
	client := &HttpService{
		lock:     new(sync.Mutex),
		groups:   make(httpGroups, gc),
		status:   serviceEnable,
		timeTick: ctx.HttpConfig.TimeTick,
		ctx:      ctx,
	}
	for _, groupConfig := range ctx.HttpConfig.Groups {
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
	client.ctx.ReloadHttpConfig()
	log.Debug("http service reloading...")

	client.status = 0
	if client.ctx.HttpConfig.Enable {
		client.status = serviceEnable
	}

	for _, group := range client.groups {
		client.groups.delete(group)
	}

	for _, groupConfig := range client.ctx.HttpConfig.Groups {
		httpGroup := newHttpGroup(client.ctx, groupConfig)
		client.lock.Lock()
		client.groups.add(httpGroup)
		client.lock.Unlock()
	}
	log.Debug("http service reloaded.")
}

func (client *HttpService) SendRaw(data []byte) bool {
	return true
}

func (client *HttpService) Name() string {
	return "http"
}
