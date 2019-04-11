package services

import (
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/g"
)

// NewTCPService TODO
func NewTCPService(ctx *g.Context) *TCPService {
	grp := newGroups(ctx)
	t := newTCPService(
		ctx,
		ctx.Config.Listen,
		SetSendAll(grp.sendAll),
		SetSendRaw(grp.asyncSend),
		SetOnConnect(grp.onConnect),
		SetOnClose(grp.close),
		SetKeepalive(grp.asyncSend),
		SetReload(grp.reload),
	)

	// 服务注册相关
	if ctx.Config.Consul.Enabled && ctx.Config.Consul.Addr != "" {
		temp := strings.Split(ctx.Config.Listen, ":")
		host := temp[0]
		port, _ := strconv.ParseInt(temp[1], 10, 32)
		svc := NewService(host, int(port), ctx.Config.Consul.Addr)
		svc.Register()
		SetOnClose(svc.Close)(t)
		SetOnConnect(svc.newConnect)(t)
		SetOnRemove(svc.disconnect)(grp)
	}

	return t
}

func newTCPService(
	ctx *g.Context,
	listen string,
	opts ...TCPServiceOption) *TCPService {

	tcp := &TCPService{
		Listen:      listen, //config.Listen,
		lock:        new(sync.Mutex),
		statusLock:  new(sync.Mutex),
		wg:          new(sync.WaitGroup),
		listener:    nil,
		ctx:         ctx,
		status:      0,
		sendAll:     make([]SendAllFunc, 0),
		sendRaw:     make([]SendRawFunc, 0),
		onConnect:   make([]OnConnectFunc, 0),
		onClose:     make([]CloseFunc, 0),
		onKeepalive: make([]KeepaliveFunc, 0),
		reload:      make([]ReloadFunc, 0),
	}
	tcp.status |= serviceEnable
	for _, f := range opts {
		f(tcp)
	}
	go tcp.keepalive()
	log.Debugf("[D] -----subscribe service init----")
	return tcp
}

// SetSendAll TODO
func SetSendAll(f SendAllFunc) TCPServiceOption {
	return func(svc *TCPService) {
		svc.sendAll = append(svc.sendAll, f)
	}
}

// SetSendRaw TODO
func SetSendRaw(f SendRawFunc) TCPServiceOption {
	return func(svc *TCPService) {
		svc.sendRaw = append(svc.sendRaw, f)
	}
}

// SetOnConnect TODO
func SetOnConnect(f OnConnectFunc) TCPServiceOption {
	return func(svc *TCPService) {
		svc.onConnect = append(svc.onConnect, f)
	}
}

// SetOnClose TODO
func SetOnClose(f CloseFunc) TCPServiceOption {
	return func(svc *TCPService) {
		svc.onClose = append(svc.onClose, f)
	}
}

// SetKeepalive TODO
func SetKeepalive(f KeepaliveFunc) TCPServiceOption {
	return func(svc *TCPService) {
		svc.onKeepalive = append(svc.onKeepalive, f)
	}
}

// SetReload TODO
func SetReload(f ReloadFunc) TCPServiceOption {
	return func(svc *TCPService) {
		svc.reload = append(svc.reload, f)
	}
}

// SendAll send event data to all connects client
func (tcp *TCPService) SendAll(table string, data []byte) bool {
	tcp.statusLock.Lock()
	if tcp.status&serviceEnable <= 0 {
		tcp.statusLock.Unlock()
		return false
	}
	tcp.statusLock.Unlock()
	log.Debugf("[D] subscribe SendAll: %s, %+v", table, string(data))
	// pack data
	packData := Pack(CMD_EVENT, data)
	for _, f := range tcp.sendAll {
		f(table, packData)
	}
	return true
}

// SendRaw send raw bytes data to all connects client
// msg is the pack frame form func: pack
func (tcp *TCPService) SendRaw(msg []byte) bool {
	tcp.statusLock.Lock()
	if tcp.status&serviceEnable <= 0 {
		tcp.statusLock.Unlock()
		return false
	}
	tcp.statusLock.Unlock()
	log.Debugf("[D] tcp sendRaw: %+v", msg)
	for _, f := range tcp.sendRaw {
		f(msg)
	}
	return true
}

// Start TODO
func (tcp *TCPService) Start() {
	tcp.statusLock.Lock()
	if tcp.status&serviceEnable <= 0 {
		tcp.statusLock.Unlock()
		return
	}
	if tcp.status&serviceClosed > 0 {
		tcp.status ^= serviceClosed
	}
	tcp.statusLock.Unlock()
	go func() {
		listen, err := net.Listen("tcp", tcp.Listen)
		if err != nil {
			log.Errorf("[E] tcp service listen with error: %+v", err)
			return
		}
		tcp.listener = &listen
		log.Infof("[I] tcp service start with: %s", tcp.Listen)
		for {
			conn, err := listen.Accept()
			select {
			case <-tcp.ctx.Ctx.Done():
				return
			default:
			}
			tcp.statusLock.Lock()
			if tcp.status&serviceClosed > 0 {
				tcp.statusLock.Unlock()
				return
			}
			tcp.statusLock.Unlock()
			if err != nil {
				log.Warnf("[W] tcp service accept with error: %+v", err)
				continue
			}
			for _, f := range tcp.onConnect {
				f(&conn)
			}
		}
	}()
}

// Close TODO
func (tcp *TCPService) Close() {
	if tcp.status&serviceClosed > 0 {
		return
	}
	log.Debugf("[D] tcp service closing, waiting for buffer send complete.")
	tcp.lock.Lock()
	defer tcp.lock.Unlock()
	if tcp.listener != nil {
		(*tcp.listener).Close()
	}
	for _, f := range tcp.onClose {
		f()
	}
	if tcp.status&serviceClosed <= 0 {
		tcp.status |= serviceClosed
	}
	log.Debugf("[D] tcp service closed.")
}

// Reload TODO
func (tcp *TCPService) Reload() {
}

func (tcp *TCPService) keepalive() {
	if tcp.status&serviceEnable <= 0 {
		return
	}
	for {
		select {
		case <-tcp.ctx.Ctx.Done():
			return
		default:
		}
		for _, f := range tcp.onKeepalive {
			f(packDataTickOk)
		}
		time.Sleep(time.Second * 3)
	}
}

// Name TODO
func (tcp *TCPService) Name() string {
	return "subscribe"
}
