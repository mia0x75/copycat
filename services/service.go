package services

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	consul "github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"
)

// 服务注册
const (
	Registered = 1 << iota
)

const (
	statusOnline  = "online"
	statusOffline = "offline"
)

// todo 这里还需要一个操作，就是，客户端接入或者断开的时候，触发更新服务的属性
// 即将当前连接数接入到consul服务，客户端做服务发现的时候，自动优先连接连接数最少的
// Service TODO
type Service struct {
	ServiceName string //service name, like: service.add
	ServiceHost string //service host, like: 0.0.0.0, 127.0.0.1
	ServiceIP   string // if ServiceHost is 0.0.0.0, ServiceIp must set,
	// like 127.0.0.1 or 192.168.9.12 or 114.55.56.168
	ServicePort int            // service port, like: 9998
	Interval    time.Duration  // interval for update ttl
	TTL         int            //check ttl
	ServiceID   string         //serviceID = fmt.Sprintf("%s-%s-%d", name, ip, port)
	client      *consul.Client //consul client
	agent       *consul.Agent  //consul agent
	status      int            // register status
	lock        *sync.Mutex    //sync lock
	handler     *consul.Session
	Kv          *consul.KV
	onleader    []OnLeaderFunc
	health      *consul.Health
	connects    int64
}

// OnLeaderFunc TODO
type OnLeaderFunc func(bool)

// ServiceOption TODO
type ServiceOption func(s *Service)

// set ttl
func TTL(ttl int) ServiceOption {
	return func(s *Service) {
		s.TTL = ttl
	}
}

// Interval set interval
func Interval(interval time.Duration) ServiceOption {
	return func(s *Service) {
		s.Interval = interval
	}
}

// ServiceIp set service ip
func ServiceIp(serviceIP string) ServiceOption {
	return func(s *Service) {
		s.ServiceIP = serviceIP
	}
}

// NewService new a service
// name: service name
// host: service host like 0.0.0.0 or 127.0.0.1
// port: service port, like 9998
// consulAddress: consul service address, like 127.0.0.1:8500
// opts: ServiceOption, like ServiceIp("127.0.0.1")
// return new service pointer
func NewService(
	host string,
	port int,
	consulAddress string,
	opts ...ServiceOption) *Service {
	s := &Service{
		ServiceName: ServiceName,
		ServiceHost: host,
		ServicePort: port,
		Interval:    time.Second * 10,
		TTL:         15,
		status:      0,
		lock:        new(sync.Mutex),
		onleader:    make([]OnLeaderFunc, 0),
		connects:    int64(0),
	}
	for _, opt := range opts {
		opt(s)
	}
	conf := &consul.Config{Scheme: "http", Address: consulAddress}
	c, err := consul.NewClient(conf)
	if err != nil {
		log.Printf("[P] %v", err)
	}
	s.client = c
	s.handler = c.Session()
	s.Kv = c.KV()
	ip := host
	if ip == "0.0.0.0" {
		if s.ServiceIP == "" {
			log.Panicf("[P] please set consul service ip")
		}
		ip = s.ServiceIP
	}
	s.ServiceID = fmt.Sprintf("%s-%s-%d", ServiceName, ip, port)
	s.agent = s.client.Agent()
	go s.updateTTL()
	s.health = s.client.Health()
	return s
}

// Deregister TODO
func (s *Service) Deregister() error {
	err := s.agent.ServiceDeregister(s.ServiceID)
	if err != nil {
		log.Errorf("[E] deregister service error: %s", err.Error())
		return err
	}
	err = s.agent.CheckDeregister(s.ServiceID)
	if err != nil {
		log.Infof("[I] deregister check error: %s", err.Error())
	}
	return err
}

func (s *Service) updateTTL() {
	ip := s.ServiceHost
	if ip == "0.0.0.0" && s.ServiceIP != "" {
		ip = s.ServiceIP
	}
	key := fmt.Sprintf("connects/%v/%v", ip, s.ServicePort)
	sessionEntry := &consul.SessionEntry{
		Behavior: consul.SessionBehaviorDelete,
		TTL:      fmt.Sprintf("%vs", s.Interval.Seconds()*3),
	}
	session, _, err := s.handler.Create(sessionEntry, nil)
	if err != nil {
		log.Panicf("[P] %+v", err)
	}
	for {
		_, _, err = s.handler.Renew(session, nil)
		if err != nil {
			log.Errorf("[E] %+v", err)
		}
		count := atomic.LoadInt64(&s.connects)
		var data = make([]byte, 8)
		binary.LittleEndian.PutUint64(data, uint64(count))
		p := &consul.KVPair{
			Key:     key,
			Value:   data, //[]byte(fmt.Sprintf("%v", count)),
			Session: session,
		}
		_, err = s.Kv.Put(p, nil)
		if err != nil {
			log.Errorf("[E] %+v", err)
		}
		if s.status&Registered <= 0 {
			time.Sleep(s.Interval)
			continue
		}
		err = s.agent.UpdateTTL(s.ServiceID, "", "passing")
		if err != nil {
			log.Errorf("[E] update ttl of service error: %s", err.Error())
		}
		time.Sleep(s.Interval)
	}
}

func (s *Service) newConnect(conn *net.Conn) {
	log.Debugf("[D] ##############service new connect##############")
	atomic.AddInt64(&s.connects, 1)
}
func (s *Service) disconnect(conn *net.Conn) {
	log.Debugf("[D] ##############service new disconnect##############")
	atomic.AddInt64(&s.connects, -1)
}

// Register TODO
func (s *Service) Register() error {
	s.lock.Lock()
	if s.status&Registered <= 0 {
		s.status |= Registered
	}
	s.lock.Unlock()
	// de-register if meet signhup
	// initial register service
	ip := s.ServiceHost
	if ip == "0.0.0.0" && s.ServiceIP != "" {
		ip = s.ServiceIP
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}
	regis := &consul.AgentServiceRegistration{
		ID:      s.ServiceID,
		Name:    s.ServiceName,
		Address: ip,
		Port:    s.ServicePort,
		Tags:    []string{hostname},
	}
	log.Debugf("[D] subscribe service register: %+v", *regis)
	err = s.agent.ServiceRegister(regis)
	if err != nil {
		return fmt.Errorf("[E] initial register service '%s' host to consul error: %s", s.ServiceName, err.Error())
	}
	// initial register service check
	check := consul.AgentServiceCheck{TTL: fmt.Sprintf("%ds", s.TTL), Status: "passing"}
	err = s.agent.CheckRegister(&consul.AgentCheckRegistration{
		ID:                s.ServiceID,
		Name:              s.ServiceName,
		ServiceID:         s.ServiceID,
		AgentServiceCheck: check,
	})
	if err != nil {
		return fmt.Errorf("[E] initial register service check to consul error: %s", err.Error())
	}
	return nil
}

// Close TODO
func (sev *Service) Close() {
	sev.Deregister()
}
