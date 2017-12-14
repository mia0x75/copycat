package unix

import (
	"net"
	log "github.com/sirupsen/logrus"
	"github.com/mia0x75/centineld/file"
	"github.com/mia0x75/centineld/binlog"
	"os"
	"context"
	"bytes"
)

func NewUnixServer() *UnixServer {
	addr := "/run/centineld/centineld.sock"
	server := &UnixServer{
		addr : addr,
	}
	return server
}

func (server *UnixServer) onConnect(c net.Conn) {
	for {
		buf := make([]byte, 512)
		nr, err := c.Read(buf)
		if err != nil {
			return
		}
		data := buf[0:nr]

		length := int(data[0]) +
			int(data[1] << 8) +
			int(data[2] << 16) +
			int(data[3] << 32)
		cmd := 	int(data[4]) +
			int(data[5] << 8)

		content := bytes.ToLower(data[6:])

		log.Debugf("unix服务收到消息：%s, %+v, %d", string(data), data, length)

		switch cmd {
		case CMD_STOP:
			log.Debug("收到退出指令，程序即将退出")
			server.clear()
			(*server.cancel)()
			server.binlog.Close()
			os.Exit(0)
		case CMD_RELOAD: {
			log.Debugf("收到重新加载指令：%s", string(content))
			server.binlog.Reload(string(content))
		}
		}
	}
}
func (server *UnixServer) clear() {
	f := file.WFile{server.addr}
	if f.Exists() {
		f.Delete()
	}
}

func (server *UnixServer) Start(binlog *binlog.Binlog, cancel *context.CancelFunc) {
	server.cancel = cancel
	server.binlog = binlog
	server.clear()
	go func() {
		log.Debug("unix服务启动，等待新的连接...")
		listen, err := net.Listen("unix", server.addr)
		if err != nil {
			log.Panicf("unix服务异常：%+v", err)
		}
		for {
			fd, err := listen.Accept()
			if err != nil {
				log.Panicf("unix服务异常：%+v", err)
			}
			go server.onConnect(fd)
		}
	}()
}
