package services

import (
	"bytes"
	"io/ioutil"
	"runtime"
	"sync"

	"github.com/gorilla/http"
	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/g"
)

func newHTTPNode(ctx *g.Context, url string) *httpNode {
	return &httpNode{
		url:       url,
		sendQueue: make(chan string, httpMaxSendQueue),
		lock:      new(sync.Mutex),
		ctx:       ctx,
		wg:        new(sync.WaitGroup),
	}
}

func (node *httpNode) asyncSend(data []byte) {
	for {
		// if cache is full, try to wait it
		if len(node.sendQueue) < cap(node.sendQueue) {
			break
		}
		log.Warnf("[W] cache full, try wait")
	}
	node.sendQueue <- string(data)
}

func (node *httpNode) wait() {
	node.wg.Wait()
}

func (node *httpNode) send(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write(data)
	_, _, r, err := http.DefaultClient.Post(node.url, nil, &buf)
	if err != nil {
		return nil, err
	}
	if r != nil {
		defer r.Close()
	}
	res, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (nodes *httpNodes) asyncSend(data []byte) {
	for _, node := range *nodes {
		node.asyncSend(data)
	}
}

func (nodes *httpNodes) sendService() {
	cpu := runtime.NumCPU() + 2
	for _, node := range *nodes {
		// 启用cpu数量的服务协程
		for i := 0; i < cpu; i++ {
			go node.clientSendService()
		}
	}
}

func (node *httpNode) clientSendService() {
	node.wg.Add(1)
	defer node.wg.Done()
	for {
		select {
		case msg, ok := <-node.sendQueue:
			if !ok {
				log.Warnf("[W] http service, sendQueue channel was closed")
				return
			}
			data, err := node.send([]byte(msg))
			if err != nil {
				log.Errorf("[E] http service node %s error: %v", node.url, err)
			}
			log.Debugf("[D] post to %s: %v return %s", msg, node.url, string(data))
		case <-node.ctx.Ctx.Done():
			l := len(node.sendQueue)
			log.Debugf("[D] ===>wait cache data post: %s left data len %d (if left data is 0, will exit) ", node.url, l)
			if l <= 0 {
				log.Debugf("[D] %s clientSendService exit", node.url)
				return
			}
		}
	}
}
