package services

import (
	"net"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/g"
)

type tcpGroups struct {
	g        []*tcpClientNode
	lock     *sync.Mutex
	ctx      *g.Context
	unique   int64
	onRemove []OnRemoveFunc
}

// OnRemoveFunc TODO
type OnRemoveFunc func(conn *net.Conn)

// TCPGroupsOptions TODO
type TCPGroupsOptions func(groups *tcpGroups)

func newGroups(ctx *g.Context, opts ...TCPGroupsOptions) *tcpGroups {
	g := &tcpGroups{
		unique:   0,
		lock:     new(sync.Mutex),
		g:        make([]*tcpClientNode, 0),
		ctx:      ctx,
		onRemove: make([]OnRemoveFunc, 0),
	}
	for _, f := range opts {
		f(g)
	}
	return g
}

// SetOnRemove TODO
func SetOnRemove(f OnRemoveFunc) TCPGroupsOptions {
	return func(groups *tcpGroups) {
		groups.onRemove = append(groups.onRemove, f)
	}
}

func (groups *tcpGroups) sendAll(table string, data []byte) bool {
	for _, node := range groups.g {
		log.Debugf("[D] topics:%+v, %v", node.topics, table)
		// 如果有订阅主题
		if MatchFilters(node.topics, table) {
			node.asyncSend(data)
		}
	}
	return true
}

func (groups *tcpGroups) remove(node *tcpClientNode) {
	for index, n := range groups.g {
		if n == node {
			groups.g = append(groups.g[:index], groups.g[index+1:]...)
			break
		}
	}
	for _, f := range groups.onRemove {
		f(node.conn)
	}
}

func (groups *tcpGroups) reload() {
	groups.close()
}

func (groups *tcpGroups) asyncSend(data []byte) {
	groups.lock.Lock()
	defer groups.lock.Unlock()
	for _, group := range groups.g {
		group.asyncSend(data)
	}
}

func (groups *tcpGroups) close() {
	for _, group := range groups.g {
		group.close()
	}
	groups.g = make([]*tcpClientNode, 0)
}

func (groups *tcpGroups) onConnect(conn *net.Conn) {
	groups.lock.Lock()
	node := newNode(groups.ctx, conn, NodeClose(groups.remove))
	groups.g = append(groups.g, node)
	groups.lock.Unlock()
	go node.onConnect()
}
