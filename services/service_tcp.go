package services

import (
	"fmt"
	"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/g"
)

// NewTCPService TODO
func NewTCPService(ctx *g.Context) *TCPService {
	group := newGroups(ctx)
	t := newTCPService(ctx,
		SetSendAll(group.sendAll),
		SetSendRaw(group.asyncSend),
		SetOnConnect(group.onConnect),
		SetOnClose(group.close),
		SetKeepalive(group.asyncSend),
		SetReload(group.reload),
	)
	return t
}

func newTCPService(ctx *g.Context, opts ...TCPServiceOption) *TCPService {
	tcp := &TCPService{
		IP:          ctx.Config.TCP.Addr,
		Port:        ctx.Config.TCP.Port,
		lock:        new(sync.Mutex),
		statusLock:  new(sync.Mutex),
		wg:          new(sync.WaitGroup),
		listener:    nil,
		ctx:         ctx,
		ServiceIP:   ctx.Config.TCP.ServiceIP,
		status:      serviceEnable,
		token:       g.GetKey(g.TOKEN_FILE),
		sendAll:     make([]SendAllFunc, 0),
		sendRaw:     make([]SendRawFunc, 0),
		onConnect:   make([]OnConnectFunc, 0),
		onClose:     make([]CloseFunc, 0),
		onKeepalive: make([]KeepaliveFunc, 0),
		reload:      make([]ReloadFunc, 0),
	}

	for _, f := range opts {
		f(tcp)
	}
	go tcp.keepalive()
	log.Debugf("[D] -----tcp service init----")
	return tcp
}

// SetSendAll TODO
func SetSendAll(f SendAllFunc) TCPServiceOption {
	return func(service *TCPService) {
		service.sendAll = append(service.sendAll, f)
	}
}

// SetSendRaw TODO
func SetSendRaw(f SendRawFunc) TCPServiceOption {
	return func(service *TCPService) {
		service.sendRaw = append(service.sendRaw, f)
	}
}

// SetOnConnect TODO
func SetOnConnect(f OnConnectFunc) TCPServiceOption {
	return func(service *TCPService) {
		service.onConnect = append(service.onConnect, f)
	}
}

// SetOnClose TODO
func SetOnClose(f CloseFunc) TCPServiceOption {
	return func(service *TCPService) {
		service.onClose = append(service.onClose, f)
	}
}

// SetKeepalive TODO
func SetKeepalive(f KeepaliveFunc) TCPServiceOption {
	return func(service *TCPService) {
		service.onKeepalive = append(service.onKeepalive, f)
	}
}

// SetReload TODO
func SetReload(f ReloadFunc) TCPServiceOption {
	return func(service *TCPService) {
		service.reload = append(service.reload, f)
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
	log.Debugf("[D] tcp SendAll: %s, %+v", table, string(data))
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
		dns := fmt.Sprintf("%s:%d", tcp.IP, tcp.Port)
		listen, err := net.Listen("tcp", dns)
		if err != nil {
			log.Errorf("[E] tcp service listen with error: %+v", err)
			return
		}
		tcp.listener = &listen
		log.Infof("[I] tcp service start with: %s", dns)
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
	tcp.statusLock.Lock()
	defer tcp.statusLock.Unlock()
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
	tcp.ctx.Reload()
	log.Debugf("[D] tcp service reload with new config：%+v", tcp.ctx.Config.TCP)
	tcp.statusLock.Lock()
	if tcp.ctx.Config.TCP.Enabled && tcp.status&serviceEnable <= 0 {
		tcp.status |= serviceEnable
	}
	if !tcp.ctx.Config.TCP.Enabled && tcp.status&serviceEnable > 0 {
		tcp.status ^= serviceEnable
	}
	tcp.statusLock.Unlock()
	for _, f := range tcp.reload {
		f()
	}
	log.Debugf("[D] tcp service restart...")
	tcp.Close()
	tcp.Start()
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
	return "tcp"
}
