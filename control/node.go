package control

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/g"
	"github.com/mia0x75/copycat/services"
)

func newNode(ctx *g.Context, conn *net.Conn, opts ...nodeOption) *TCPClientNode {
	node := &TCPClientNode{
		conn:    conn,
		recvBuf: make([]byte, 0),
		status:  tcpNodeOnline,
		ctx:     ctx,
		lock:    new(sync.Mutex),
	}
	if len(opts) > 0 {
		for _, f := range opts {
			f(node)
		}
	}
	node.setReadDeadline(time.Now().Add(time.Second * 3))
	return node
}

func nodeStop(s StopFunc) nodeOption {
	return func(n *TCPClientNode) {
		n.stop = s
	}
}

func nodeReload(s ReloadFunc) nodeOption {
	return func(n *TCPClientNode) {
		n.reload = s
	}
}

func nodeShowMembers(s ShowMemberFunc) nodeOption {
	return func(n *TCPClientNode) {
		n.showmember = s
	}
}

func (node *TCPClientNode) close() {
	node.lock.Lock()
	defer node.lock.Unlock()
	if node.status&tcpNodeOnline <= 0 {
		return
	}
	if node.status&tcpNodeOnline > 0 {
		node.status ^= tcpNodeOnline
		(*node.conn).Close()
	}
}

func (node *TCPClientNode) send(data []byte) (int, error) {
	if node.status&tcpNodeOnline <= 0 {
		return 0, errNodeOffline
	}
	(*node.conn).SetWriteDeadline(time.Now().Add(time.Second * 3))
	return (*node.conn).Write(data)
}

func (node *TCPClientNode) setReadDeadline(t time.Time) {
	(*node.conn).SetReadDeadline(t)
}

func (node *TCPClientNode) onMessage(msg []byte) {
	log.Debugf("[D] receive msg: %+v", msg)
	node.recvBuf = append(node.recvBuf, msg...)
	for {
		size := len(node.recvBuf)
		if size < 6 {
			return
		}
		clen := int(node.recvBuf[0]) | int(node.recvBuf[1])<<8 |
			int(node.recvBuf[2])<<16 | int(node.recvBuf[3])<<24
		log.Debugf("[D] len: %+v", clen)
		if len(node.recvBuf) < clen+4 {
			return
		}
		cmd := int(node.recvBuf[4]) | int(node.recvBuf[5])<<8
		log.Debugf("[D] cmd: %v", cmd)
		if !hasCmd(cmd) {
			log.Errorf("[E] cmd %d does not exists, data: %v", cmd, node.recvBuf)
			node.recvBuf = make([]byte, 0)
			return
		}
		content := node.recvBuf[6 : clen+4]
		switch cmd {
		case CMD_TICK:
			node.send(packDataTickOk)
		case CMD_STOP:
			log.Debugf("[D] receive stop cmd")
			node.stop()
			node.send(services.Pack(CMD_STOP, []byte("ok")))
		case CMD_RELOAD:
			node.reload(string(content))
			node.send(services.Pack(CMD_RELOAD, []byte("ok")))
		case CMD_SHOW_MEMBERS:
			members := node.showmember()
			node.send(services.Pack(CMD_SHOW_MEMBERS, []byte(members)))
		default:
			node.send(services.Pack(CMD_ERROR, []byte(fmt.Sprintf("tcp service does not support cmd: %d", cmd))))
			node.recvBuf = make([]byte, 0)
			return
		}
		node.recvBuf = append(node.recvBuf[:0], node.recvBuf[clen+4:]...)
	}
}

func (node *TCPClientNode) readMessage() {
	var readBuffer [tcpDefaultReadBufferSize]byte
	// 设定3秒超时，如果添加到分组成功，超时限制将被清除
	for {
		size, err := (*node.conn).Read(readBuffer[0:])
		if err != nil {
			if err != io.EOF {
				log.Warnf("[W] tcp node %s disconnect with error: %v", (*node.conn).RemoteAddr().String(), err)
			} else {
				log.Debugf("[D] tcp node %s disconnect with error: %v", (*node.conn).RemoteAddr().String(), err)
			}
			node.close()
			return
		}
		log.Debugf("[D] receive msg: %v, %+v", size, readBuffer[:size])
		node.onMessage(readBuffer[:size])
	}
}
