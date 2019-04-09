package control

import (
	"net"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/g"
)

// NewControl TODO
func NewControl(ctx *g.Context, opts ...Option) *TCPService {
	tcp := &TCPService{
		Address:  ctx.Config.Control.Listen,
		lock:     new(sync.Mutex),
		wg:       new(sync.WaitGroup),
		listener: nil,
		ctx:      ctx,
		token:    g.GetKey(g.TOKEN_FILE),
	}
	for _, f := range opts {
		f(tcp)
	}
	return tcp
}

// ShowMember TODO
func ShowMember(f ShowMemberFunc) Option {
	return func(tcp *TCPService) {
		tcp.showmember = f
	}
}

// Reload TODO
func Reload(f ReloadFunc) Option {
	return func(tcp *TCPService) {
		tcp.reload = f
	}
}

// Stop TODO
func Stop(f StopFunc) Option {
	return func(tcp *TCPService) {
		tcp.stop = f
	}
}

// Start TODO
func (tcp *TCPService) Start() {
	go func() {
		listen, err := net.Listen("tcp", tcp.Address)
		if err != nil {
			log.Errorf("[E] tcp service listen with error: %+v", err)
			return
		}
		tcp.listener = &listen
		for {
			conn, err := listen.Accept()
			select {
			case <-tcp.ctx.Ctx.Done():
				return
			default:
			}
			if err != nil {
				log.Warnf("[W] tcp service accept with error: %+v", err)
				continue
			}
			node := newNode(tcp.ctx, &conn, nodeStop(tcp.stop), nodeReload(tcp.reload), nodeShowMembers(tcp.showmember))
			go node.readMessage()
		}
	}()
}

// Close TODO
func (tcp *TCPService) Close() {
	log.Debugf("[D] tcp service closing, waiting for buffer send complete.")
	tcp.lock.Lock()
	defer tcp.lock.Unlock()
	if tcp.listener != nil {
		(*tcp.listener).Close()
	}
	log.Debugf("[D] tcp service closed.")
}
