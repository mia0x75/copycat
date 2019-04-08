package services

import (
	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/g"
)

func newHttpGroup(ctx *g.Context, groupConfig *g.HTTPGroupConfig) *httpGroup {
	group := &httpGroup{
		name:   groupConfig.Name,
		filter: groupConfig.Filter,
		nodes:  make(httpNodes, len(groupConfig.Endpoints)),
	}
	for i, url := range groupConfig.Endpoints {
		group.nodes[i] = newHttpNode(ctx, url)
	}
	return group
}

func (group *httpGroup) match(table string) bool {
	if len(group.nodes) <= 0 || !MatchFilters(group.filter, table) {
		return false
	}
	return true
}

func (group *httpGroup) asyncSend(data []byte) {
	for _, cnode := range group.nodes {
		log.Debugf("[D] http send broadcast: %s=>%s", cnode.url, string(data))
		cnode.asyncSend(data)
	}
}

func (group *httpGroup) wait() {
	for _, node := range group.nodes {
		node.wait()
	}
}

func (group *httpGroup) sendService() {
	group.nodes.sendService()
}

func (groups *httpGroups) wait() {
	for _, group := range *groups {
		group.wait()
	}
}

func (groups *httpGroups) sendService() {
	for _, group := range *groups {
		group.sendService()
	}
}

func (groups *httpGroups) asyncSend(table string, data []byte) {
	for _, group := range *groups {
		if group.match(table) {
			group.asyncSend(data)
		}
	}
}

func (groups *httpGroups) add(group *httpGroup) {
	(*groups)[group.name] = group
}

func (groups *httpGroups) delete(group *httpGroup) {
	delete((*groups), group.name)
}
