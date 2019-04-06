package services

import (
	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/nova/app"
)

func newHttpGroup(ctx *app.Context, groupConfig app.HttpNodeConfig) *httpGroup {
	group := &httpGroup{
		name:   groupConfig.Name,
		filter: groupConfig.Filter,
		nodes:  make(httpNodes, len(groupConfig.Nodes)),
	}
	for i, url := range groupConfig.Nodes {
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
		log.Debugf("http send broadcast: %s=>%s", cnode.url, string(data))
		cnode.asyncSend(data)
	}
}

func (group *httpGroup) wait() {
	for _, node := range group.nodes {
		node.wait()
	}
}

func (group *httpGroup) sendService() {
	//cpu := runtime.NumCPU() + 2
	group.nodes.sendService()
	//for _, node := range group.nodes {
	//	// 启用cpu数量的服务协程
	//	for i := 0; i < cpu; i++ {
	//		go node.clientSendService()
	//	}
	//}
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
