package services

import (
	"net"
	"sync"

	"github.com/mia0x75/copycat/g"
)

type tcpGroups struct {
	g    map[string]*tcpGroup
	lock *sync.Mutex
	ctx  *g.Context
}

func newGroups(ctx *g.Context) *tcpGroups {
	g := &tcpGroups{
		lock: new(sync.Mutex),
		g:    make(map[string]*tcpGroup),
		ctx:  ctx,
	}
	for _, group := range ctx.Config.TCP.Groups {
		tcpGroup := newTCPGroup(group)
		g.add(tcpGroup)
	}
	return g
}

func (groups *tcpGroups) reload() {
	groups.close()
	for _, group := range groups.ctx.Config.TCP.Groups {
		tcpGroup := newTCPGroup(group)
		groups.add(tcpGroup)
	}
}

func (groups *tcpGroups) add(group *tcpGroup) {
	groups.lock.Lock()
	defer groups.lock.Unlock()
	groups.g[group.name] = group
}

func (groups *tcpGroups) delete(group *tcpGroup) {
	groups.lock.Lock()
	defer groups.lock.Unlock()
	delete(groups.g, group.name)
}

func (groups *tcpGroups) hasName(findName string) bool {
	groups.lock.Lock()
	defer groups.lock.Unlock()
	_, ok := groups.g[findName]
	return ok
}

func (groups *tcpGroups) asyncSend(data []byte) {
	groups.lock.Lock()
	defer groups.lock.Unlock()
	for _, group := range groups.g {
		group.asyncSend(data)
	}
}

func (groups *tcpGroups) close() {
	groups.lock.Lock()
	defer groups.lock.Unlock()
	for key, group := range groups.g {
		group.close()
		delete(groups.g, key)
	}
}

func (groups *tcpGroups) removeNode(node *tcpClientNode) {
	groups.lock.Lock()
	defer groups.lock.Unlock()
	if group, found := groups.g[node.group]; found {
		group.remove(node)
	}
}

func (groups *tcpGroups) addNode(node *tcpClientNode, groupName string) bool {
	groups.lock.Lock()
	group, found := groups.g[groupName]
	groups.lock.Unlock()
	if !found || groupName == "" {
		return false
	}
	group.append(node)
	return true
}

func (groups *tcpGroups) sendAll(table string, data []byte) bool {
	for _, group := range groups.g {
		if group.match(table) {
			group.asyncSend(data)
		}
	}
	return true
}

func (groups *tcpGroups) onConnect(conn *net.Conn) {
	node := newNode(groups.ctx, conn, NodeClose(groups.removeNode), NodePro(groups.addNode))
	go node.onConnect()
}
