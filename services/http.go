package services

import (
	log "github.com/sirupsen/logrus"
	"time"
	"sync/atomic"
	"sync"
	"github.com/mia0x75/centineld/http"
	"regexp"
	"context"
)

// 创建一个新的http服务
func NewHttpService() *HttpService {
	config, _ := getHttpConfig()
	log.Debugf("HttpService::NewHtpService - http服务配置：%+v", config)
	if !config.Enable {
		return &HttpService{enable:config.Enable}
	}
	glen   := len(config.Groups)
	client := &HttpService {
		//send_queue         : make(chan []byte, TCP_MAX_SEND_QUEUE),
		lock               : new(sync.Mutex),
		groups             : make([][]*httpNode, glen),
		groups_filter      : make([][]string, glen),
		send_failure_times : int64(0),
		enable             : config.Enable,
		time_tick          : config.TimeTick,
	}
	index := 0
	for _, v := range config.Groups {
		nodes_len := len(v.Nodes)
		client.groups[index]        = make([]*httpNode, nodes_len)
		client.groups_filter[index] = make([]string, len(v.Filter))
		client.groups_filter[index] = append(client.groups_filter[index][:0], v.Filter...)
		log.Debug("HttpService::NewHtpService - http服务过滤器", client.groups_filter[index])
		for i := 0; i < nodes_len; i++ {
			client.groups[index][i] = &httpNode{
				url                : v.Nodes[i][0],
				send_queue         : make(chan string, TCP_MAX_SEND_QUEUE),
				send_times         : int64(0),
				send_failure_times : int64(0),
				is_down            : false,
				lock               : new(sync.Mutex),
				failure_times_flag : int32(0),
				cache_is_init      : false,
			}
		}
		index++
	}
	return client
}

// 开始服务
func (client *HttpService) Start() {
	if !client.enable {
		return
	}
	for _, clients :=range client.groups {
		for _, h := range clients {
			go client.clientSendService(h)
		}
	}
}

// 初始化节点缓冲区，这个缓冲区用于存放发送失败的数据，最多HTTP_CACHE_LEN条
func (client *HttpService) cacheInit(node *httpNode) {
	if node.cache_is_init {
		return
	}
	log.Infof("http服务初始化失败重试使用的cache")
	node.cache = make([][]byte, HTTP_CACHE_LEN)
	for k := 0; k < HTTP_CACHE_LEN; k++ {
		node.cache[k] = make([]byte, HTTP_CACHE_BUFFER_SIZE)
	}
	node.cache_is_init = true
	node.cache_index   = 0
	node.cache_full    = false
}

// 添加数据到缓冲区
func (client *HttpService) addCache(node *httpNode, msg []byte) {
	node.cache[node.cache_index] = append(node.cache[node.cache_index][:0], msg...)
	node.cache_index++
	log.Debugf("HttpService::addCache - http服务添加cache数据", node.cache_index)
	if node.cache_index >= HTTP_CACHE_LEN {
		node.cache_index = 0;
		node.cache_full = true
	}
}

// 尝试对失败的数据进行重发
func (client *HttpService) sendCache(node *httpNode) {
	if node.cache_index > 0 {
		//保持时序
		if node.cache_full {
			for j := node.cache_index; j < HTTP_CACHE_LEN; j++ {
				//重发
				log.Warn( "http服务数据重发(缓冲区满)", node.cache_index)
				node.send_queue <- string(node.cache[j])
			}
			node.cache_full = false
		}
		for j := 0; j < node.cache_index; j++ {
			//重发
			log.Warnf("http服务数据重发")
			node.send_queue <- string(node.cache[j])
			node.cache_index--
		}
	}
}

// 节点故障检测与恢复服务
func (client *HttpService) errorCheckService(node *httpNode) {
	for {
		node.lock.Lock()
		if node.is_down {
			// 发送空包检测
			// post默认3秒超时，所以这里不会死锁
			log.Debugf("HttpService::errorCheckService - http服务-故障节点探测：%s", node.url)
			_, err := http.Post(node.url,[]byte{byte(0)})
			if err == nil {
				//重新上线
				node.is_down = false
				log.Warn("http服务节点恢复", node.url)
				//对失败的cache进行重发
				client.sendCache(node)
			} else {
				log.Errorf("http服务-故障节点发生错误：%+v", err)
			}
		}
		node.lock.Unlock()
		time.Sleep(time.Second * time.Duration(client.time_tick))
		select{
		case <-(*client.ctx).Done():
			log.Debugf("http服务errorCheckService退出：%s", node.url)
			return
		default:
		}
	}
}

// 节点服务协程
func (client *HttpService) clientSendService(node *httpNode) {
	go client.errorCheckService(node)
	for {
		select {
		case  msg, ok := <-node.send_queue:
			if !ok {
				log.Warnf("http服务-发送消息channel通道关闭")
				return
			}
			if !node.is_down {
				atomic.AddInt64(&node.send_times, int64(1))
				log.Debug("http服务 post数据到url：",
					node.url, string(msg))
				data, err := http.Post(node.url, []byte(msg))
				if (err != nil) {
					atomic.AddInt64(&client.send_failure_times, int64(1))
					atomic.AddInt64(&node.send_failure_times, int64(1))
					atomic.AddInt32(&node.failure_times_flag, int32(1))
					failure_times := atomic.LoadInt32(&node.failure_times_flag)
					// 如果连续3次错误，标志位故障
					if failure_times >= 3 {
						//发生故障
						log.Warn(node.url, "http服务发生错误，下线节点", node.url)
						node.lock.Lock()
						node.is_down = true
						node.lock.Unlock()
					}
					log.Warn("http服务失败url和次数：", node.url, node.send_failure_times)
					client.cacheInit(node)
					client.addCache(node, []byte(msg))
				} else {
					node.lock.Lock()
					if node.is_down {
						node.is_down = false
					}
					node.lock.Unlock()
					failure_times := atomic.LoadInt32(&node.failure_times_flag)
					//恢复即时清零故障计数
					if failure_times > 0 {
						atomic.StoreInt32(&node.failure_times_flag, 0)
					}
					//对失败的cache进行重发
					client.sendCache(node)
				}
				log.Debug("http服务 post返回值：", node.url, string(data))
			} else {
				// 故障节点，缓存需要发送的数据
				// 这里就需要一个map[string][10000][]byte，最多缓存10000条
				// 保持最新的10000条
				client.addCache(node, []byte(msg))
			}
			case <-(*client.ctx).Done():
				log.Debugf("http服务clientSendService退出：%s", node.url)
				return
		}
	}
}

// 对外的广播发送接口
func (client *HttpService) SendAll(msg []byte) bool {
    if !client.enable {
        return false
    }
	//if len(client.send_queue) >= cap(client.send_queue) {
    //    log.Warn("http服务发送缓冲区满...")
    //    return false
    //}
	log.Debug("http服务-发送广播：", string(msg))
	client.lock.Lock()
	for index, clients := range client.groups {
		// 如果分组里面没有客户端连接，跳过
		if len(clients) <= 0 {
			continue
		}
		// 分组的模式
		filter := client.groups_filter[index]
		flen   := len(filter)
		//2字节长度
		table_len := int(msg[0]) + int(msg[1] << 8);
		table := string(msg[2:table_len+2])
		log.Debugf("HttpService::SendAll - http服务事件发生的数据表：%d, %s", table_len, table)
		//分组过滤
		//log.Println(filter)
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
			log.Debug("HttpService::SendAll - http服务发送广播消息：", conn.url, string(msg[table_len+2:]))
			if len(conn.send_queue) >= cap(conn.send_queue) {
				log.Warnf("http服务发送缓冲区满：%s, %s", conn.url, string(msg[table_len+2:]))
				continue
			}
			conn.send_queue <- string(msg[table_len+2:])
		}
	}
	client.lock.Unlock()
    return true
}

func (client *HttpService) Close() {
	log.Debug("http服务退出...")
}

func (tcp *HttpService) SetContext(ctx *context.Context) {
	tcp.ctx = ctx
}

func (tcp *HttpService) Reload() {
	config, _ := getHttpConfig()
	log.Debug("http服务reload...")
	tcp.enable = config.Enable
	for i, _ := range tcp.groups {
		tcp.groups[i] = make([]*httpNode, 0)
		tcp.groups_filter[i] = make([]string, 0)
	}

	index := 0
	for _, v := range config.Groups {
		nodes_len := len(v.Nodes)
		tcp.groups[index]        = make([]*httpNode, nodes_len)
		tcp.groups_filter[index] = make([]string, len(v.Filter))
		tcp.groups_filter[index] = append(tcp.groups_filter[index][:0], v.Filter...)
		log.Debug("http服务过滤器", tcp.groups_filter[index])
		for i := 0; i < nodes_len; i++ {
			tcp.groups[index][i] = &httpNode{
				url                : v.Nodes[i][0],
				send_queue         : make(chan string, TCP_MAX_SEND_QUEUE),
				send_times         : int64(0),
				send_failure_times : int64(0),
				is_down            : false,
				lock               : new(sync.Mutex),
				failure_times_flag : int32(0),
				cache_is_init      : false,
			}
		}
		index++
	}
	log.Debug("http服务reload...end")
}
