package services

import (
	"fmt"
	"net"
	log "github.com/sirupsen/logrus"
	"time"
	"sync/atomic"
	"sync"
	"regexp"
	"context"
)

func NewTcpService() *TcpService {
	config, _ := getTcpConfig()
	tcp := &TcpService {
		Ip                 : config.Tcp.Listen,
		Port               : config.Tcp.Port,
		clients_count      : int32(0),
		lock               : new(sync.Mutex),
		groups             : make(map[string][]*tcpClientNode),
		groups_filter      : make(map[string] []string),
		recv_times         : 0,
		send_times         : 0,
		send_failure_times : 0,
		enable             : config.Enable,
	}
	for _, v := range config.Groups {
		flen := len(v.Filter)
		var con [TCP_DEFAULT_CLIENT_SIZE]*tcpClientNode
		tcp.groups[v.Name]        = con[:0]
		tcp.groups_filter[v.Name] = make([]string, flen)
		tcp.groups_filter[v.Name] = append(tcp.groups_filter[v.Name][:0], v.Filter...)
	}
	return tcp
}

// 对外的广播发送接口
func (tcp *TcpService) SendAll(msg []byte) bool {
	if !tcp.enable {
		return false
	}
	log.Debugf("TcpService::SendAll - %s", string(msg))
	cc := atomic.LoadInt32(&tcp.clients_count)
	if cc <= 0 {
		log.Debugf("TcpService::SendAll - No client(s) connected the server.")
		return false
	}
	//if len(tcp.send_queue) >= cap(tcp.send_queue) {
	//	log.Warn("tcp服务-发送缓冲区满")
	//	return false
	//}
	table_len := int(msg[0]) + int(msg[1] << 8)
	table     := string(msg[2:table_len+2])
	//tcp.send_queue <-
	//pack_msg := tcp.pack2(CMD_EVENT, msg[table_len+2:], msg[:table_len+2])

	tcp.lock.Lock()
	for group_name, clients := range tcp.groups {
		// 如果分组里面没有客户端连接，跳过
		if len(clients) <= 0 {
			continue
		}
		// 分组的模式
		filter := tcp.groups_filter[group_name]
		flen   := len(filter)
		//2字节长度
		//table_len := int(msg[0]) + int(msg[1] << 8);
		//msg[2:table_len+2])
		log.Debug("TcpService::SendAll - tcp服务-数据表：", table_len, table)
		log.Debug("TcpService::SendAll - tcp服务-发送广播消息：", msg[table_len+2:])
		if flen > 0 {
			is_match := false
			for _, f := range filter {
				match, err := regexp.MatchString(f, table)
				if err != nil {
					continue
				}
				if match {
					is_match = true
					break
				}
			}
			if !is_match {
				continue
			}
		}
		for _, conn := range clients {
			if !conn.is_connected {
				continue
			}
			log.Debug("TcpService::SendAll - tcp服务-发送广播消息")
			if len(conn.send_queue) >= cap(conn.send_queue) {
				log.Warnf("tcp服务-发送缓冲区满：%s", (*conn.conn).RemoteAddr().String())
				continue
			}
			conn.send_queue <- tcp.pack(CMD_EVENT, string(msg[table_len+2:]))//msg[table_len+2:]
		}
	}
	tcp.lock.Unlock()
	return true
}

// 数据封包
func (tcp *TcpService) pack(cmd int, msg string) []byte {
	m := []byte(msg)
	l := len(m)
	r := make([]byte, l + 6)
	cl := l + 2
	r[0] = byte(cl)
	r[1] = byte(cl >> 8)
	r[2] = byte(cl >> 16)
	r[3] = byte(cl >> 32)
	r[4] = byte(cmd)
	r[5] = byte(cmd >> 8)
	copy(r[6:], m)
	return r
}

// 掉线回调
func (tcp *TcpService) onClose(conn *tcpClientNode) {
	if conn.group == "" {
		tcp.lock.Lock()
		conn.is_connected = false
		close(conn.send_queue)
		tcp.lock.Unlock()
		return
	}
	//移除conn
	//查实查找位置
	tcp.lock.Lock()
	close(conn.send_queue)
	for index, con := range tcp.groups[conn.group] {
		if con.conn == conn.conn {
			con.is_connected = false
			tcp.groups[conn.group] = append(tcp.groups[conn.group][:index], tcp.groups[conn.group][index+1:]...)
			break
		}
	}
	tcp.lock.Unlock()
	atomic.AddInt32(&tcp.clients_count, int32(-1))
	log.Infof("tcp服务-当前连输的客户端：%d, %s", len(tcp.groups[conn.group]), tcp.groups[conn.group])
	log.Infof("total clients: %d.", tcp.clients_count)
}

// 客户端服务协程，一个客户端一个
func (tcp *TcpService) clientSendService(node *tcpClientNode) {
	for {
		if !node.is_connected {
			log.Info("tcp服务-clientSendService退出")
			return
		}
		select {
		case  msg, ok := <-node.send_queue:
			if !ok {
				log.Info("tcp服务-发送消息channel通道关闭")
				return
			}
			(*node.conn).SetWriteDeadline(time.Now().Add(time.Second*1))
			size, err := (*node.conn).Write(msg)
			atomic.AddInt64(&node.send_times, int64(1))
			if (size <= 0 || err != nil) {
				atomic.AddInt64(&tcp.send_failure_times, int64(1))
				atomic.AddInt64(&node.send_failure_times, int64(1))
				log.Warn("tcp服务-失败次数：", (*node.conn).RemoteAddr().String(), node.send_failure_times)
			}
		}
	}
}

// 连接成功回调
func (tcp *TcpService) onConnect(conn net.Conn) {
	log.Infof("client %s connected.", conn.RemoteAddr().String())
	cnode := &tcpClientNode {
		conn               : &conn,
		is_connected       : true,
		send_queue         : make(chan []byte, TCP_MAX_SEND_QUEUE),
		send_failure_times : 0,
		connect_time       : time.Now().Unix(),
		send_times         : int64(0),
		recv_buf           : make([]byte, TCP_RECV_DEFAULT_SIZE),
		recv_bytes         : 0,
		group              : "",
	}
	go tcp.clientSendService(cnode)
	var read_buffer [TCP_DEFAULT_READ_BUFFER_SIZE]byte
	// 设定3秒超时，如果添加到分组成功，超时限制将被清除
	conn.SetReadDeadline(time.Now().Add(time.Second*3))
	for {
		buf := read_buffer[:TCP_DEFAULT_READ_BUFFER_SIZE]
		//清空旧数据 memset
		for k,_:= range buf {
			buf[k] = byte(0)
		}
		size, err := conn.Read(buf)
		if err != nil {
			log.Warn("tcp服务-连接发生错误: ", conn.RemoteAddr().String(), err)
			tcp.onClose(cnode);
			conn.Close();
			return
		}
		//log.Infof("Total clients: %d.", tcp.clients_count)
		log.Debugf("TcpService::onConnect - tcp服务-收到消息",size,"字节：", buf[:size], string(buf))
		atomic.AddInt64(&tcp.recv_times, int64(1))
		cnode.recv_bytes += size
		tcp.onMessage(cnode, buf, size)

		select {
			case <-(*tcp.ctx).Done():
				log.Debugf("tcp服务-onConnect退出")
				return
			default:
		}
	}
}

// 收到消息回调函数
func (tcp *TcpService) onMessage(conn *tcpClientNode, msg []byte, size int) {
	conn.recv_buf = append(conn.recv_buf[:conn.recv_bytes - size], msg[0:size]...)
	for {
		clen := len(conn.recv_buf)
		if clen < 6 {
			return
		} else if clen > TCP_RECV_DEFAULT_SIZE {
			// 清除所有的读缓存，防止发送的脏数据不断的累计
			conn.recv_buf = make([]byte, TCP_RECV_DEFAULT_SIZE)
			log.Info("tcp服务-新建缓冲区")
			return
		}
		//4字节长度
		content_len := int(conn.recv_buf[0]) +
			int(conn.recv_buf[1] << 8) +
			int(conn.recv_buf[2] << 16) +
			int(conn.recv_buf[3] << 32)
		//2字节 command
		cmd := int(conn.recv_buf[4]) + int(conn.recv_buf[5] << 8)
		switch cmd {
		case CMD_SET_PRO:
			log.Debug("TcpService::onMessage - tcp服务-收到注册分组消息")
			//内容长度+4字节的前缀（存放内容长度的数值）
			group := string(conn.recv_buf[6:content_len + 4])
			tcp.lock.Lock()
			if _, ok := tcp.groups[group]; !ok {
				conn.send_queue <- tcp.pack(CMD_ERROR, fmt.Sprintf("tcp服务-组不存在：%s", group))
				tcp.lock.Unlock()
				return
			}
			(*conn.conn).SetReadDeadline(time.Time{})
			conn.send_queue <- tcp.pack(CMD_SET_PRO, "ok")
			conn.group  = group
			tcp.groups[group] = append(tcp.groups[group], conn)
			atomic.AddInt32(&tcp.clients_count, int32(1))
			tcp.lock.Unlock()
		case CMD_TICK:
			conn.send_queue <- tcp.pack(CMD_TICK, "ok")
		//心跳包
		default:
			conn.send_queue <- tcp.pack(CMD_ERROR, fmt.Sprintf("不支持的指令：%d", cmd))
		}
		//数据移动
		conn.recv_buf = append(conn.recv_buf[:0], conn.recv_buf[content_len + 4:conn.recv_bytes]...)
		conn.recv_bytes = conn.recv_bytes - content_len - 4
	}
}

func (tcp *TcpService) Start() {
	if !tcp.enable {
		return
	}
	go func() {
		//建立socket，监听端口
		dns := fmt.Sprintf("%s:%d", tcp.Ip, tcp.Port)
		listen, err := net.Listen("tcp", dns)
		if err != nil {
			log.Error("TcpService::Start - tcp服务发生错误：", err)
			return
		}
		tcp.listener = &listen
		log.Infof("server created on IP: '0.0.0.0'")
		log.Infof("/usr/sbin/centineld: ready for connections.")
		error_times := 0
		for {
			conn, err := listen.Accept()
			if error_times > 2 {
				break
			}
			if err != nil {
				log.Warn("TcpService::Start - tcp服务发生错误：", err)
				error_times++
				continue
			}
			error_times = 0
			go tcp.onConnect(conn)
		}
	} ()
}

func (tcp *TcpService) Close() {
	log.Debug("TcpService::Close - closing...")
	tcp.lock.Lock()
	(*tcp.listener).Close()
	for _, v := range tcp.groups {
		for _, client := range v {
			(*client.conn).Close()
			client.is_connected = false
		}
	}
	tcp.lock.Unlock()
	log.Debug("tcp服务退出...end")
}

func (tcp *TcpService) SetContext(ctx *context.Context) {
	tcp.ctx = ctx
}

// 重新加载服务
func (tcp *TcpService) Reload() {
	log.Debug("tcp服务reload...")
	config, _ := getTcpConfig()
	log.Debugf("新配置：%+v", config)
	tcp.enable = config.Enable
	restart := false
	if tcp.Ip != config.Tcp.Listen || tcp.Port != config.Tcp.Port {
		log.Debugf("tcp服务，更新ip或服务端口，需要重启tcp服务，旧：%s, 新：%s",
			fmt.Sprintf("%s:%d", tcp.Ip, tcp.Port),
			fmt.Sprintf("%s:%d", config.Tcp.Listen, config.Tcp.Port))
		tcp.Ip = config.Tcp.Listen
		tcp.Port = config.Tcp.Port
		restart                = true
		tcp.clients_count      = 0
		tcp.recv_times         = 0
		tcp.send_times         = 0
		tcp.send_failure_times = 0

		//如果端口和监听ip发生变化，则需要清理所有的客户端连接
		for k, v := range tcp.groups {
			for _, conn := range v {
				log.Debugf("tcp服务关闭连接：%s", (*conn.conn).RemoteAddr().String())
				(*conn.conn).Close()
			}
			log.Debugf("tcp服务删除分组：%s", k)
			delete(tcp.groups, k)
		}
		for k, _ := range tcp.groups_filter {
			log.Debugf("tcp服务删除分组过滤器：%s", k)
			delete(tcp.groups_filter, k)
		}
	} else {
		//如果监听ip和端口没发生变化，则需要diff分组
		//检测是否发生删除和新增分组
		for _, v := range config.Groups {

			in_array := false
			for k, _ := range tcp.groups {
				if k == v.Name {
					in_array = true
					break
				}
			}

			// 新增的分组
			if !in_array {
				log.Debugf("tcp服务新增分组：%s", v.Name)
				flen := len(v.Filter)
				var con [TCP_DEFAULT_CLIENT_SIZE]*tcpClientNode
				tcp.groups[v.Name] = con[:0]
				tcp.groups_filter[v.Name] = make([]string, flen)
				tcp.groups_filter[v.Name] = append(tcp.groups_filter[v.Name][:0], v.Filter...)
			}
		}

		for k, conns := range tcp.groups {
			in_array := false
			for _, v := range config.Groups {
				if k == v.Name {
					in_array = true
					break
				}
			}
			//被删除的分组
			if !in_array {
				log.Debugf("tcp服务移除的分组：%s", k)
				for _, conn := range conns {
					log.Debugf("tcp服务关闭连接：%s", (*conn.conn).RemoteAddr().String())
					(*conn.conn).Close()
				}
				delete(tcp.groups, k)
			}
		}
	}

	if restart {
		log.Debugf("tcp服务重启...")
		tcp.Close()
		tcp.Start()
	}
}
