package services

import (
	"sync"

	"github.com/mia0x75/copycat/g"
)

func newTCPGroup(group *g.TCPGroupConfig) *tcpGroup {
	g := &tcpGroup{
		name:   group.Name,
		filter: group.Filter,
		nodes:  nil,
		lock:   new(sync.Mutex),
	}
	return g
}

func (g *tcpGroup) match(table string) bool {
	return MatchFilters(g.filter, table)
}

func (g *tcpGroup) append(node *tcpClientNode) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.nodes = append(g.nodes, node)
}

func (g *tcpGroup) remove(node *tcpClientNode) {
	g.lock.Lock()
	defer g.lock.Unlock()
	for index, n := range g.nodes {
		if n == node {
			g.nodes = append(g.nodes[:index], g.nodes[index+1:]...)
			break
		}
	}

}

func (g *tcpGroup) close() {
	for _, node := range g.nodes {
		node.close()
	}
}

func (g *tcpGroup) asyncSend(data []byte) {
	for _, node := range g.nodes {
		node.asyncSend(data)
	}
}
