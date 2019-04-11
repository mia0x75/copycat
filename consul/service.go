package consul

import (
	"fmt"
	"sync"

	"github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/g"
)

// 服务注册
const (
	Registered = 1 << iota
)
const (
	statusOnline  = "online"
	statusOffline = "offline"
)

// ServiceMember TODO
type ServiceMember struct {
	IsLeader  bool
	ServiceID string
	Status    string
	ServiceIP string
	Port      int
}

// Service TODO
type Service struct {
	ServiceName string //service name, like: service.add
	ServiceIP   string // if ServiceHost is 0.0.0.0, ServiceIp must set,
	// like 127.0.0.1 or 192.168.9.12 or 114.55.56.168
	ServicePort int         // service port, like: 9998
	ServiceID   string      //serviceID = fmt.Sprintf("%s-%s-%d", name, ip, port)
	agent       *api.Agent  //consul agent
	status      int         // register status
	lock        *sync.Mutex //sync lock
	leader      bool
	TTL         int
}

// IService TODO
type IService interface {
	Deregister() error
	Register() error
	UpdateTTL() error
	SetLeader(bool)
}

// ServiceOption TODO
type ServiceOption func(s *Service)

// NewService new a service
// name: service name
// host: service host like 0.0.0.0 or 127.0.0.1
// port: service port, like 9998
// consulAddress: consul service address, like 127.0.0.1:8500
// opts: ServiceOption, like ServiceIp("127.0.0.1")
// return new service pointer
func NewService(
	agent *api.Agent, //127.0.0.1:8500
	name string,
	host string,
	port int,
	opts ...ServiceOption,
) IService {
	sev := &Service{
		ServiceName: name,
		ServiceIP:   host,
		ServicePort: port,
		TTL:         15,
		status:      0,
		leader:      false,
		lock:        new(sync.Mutex),
		ServiceID:   fmt.Sprintf("%s-%s-%d", name, host, port),
		agent:       agent,
	}
	for _, opt := range opts {
		opt(sev)
	}
	return sev
}

// SetLeader TODO
func (sev *Service) SetLeader(leader bool) {
	sev.leader = leader
}

// Deregister TODO
func (sev *Service) Deregister() error {
	err := sev.agent.ServiceDeregister(sev.ServiceID)
	if err != nil {
		log.Errorf("[E] deregister service error: %s", err.Error())
		return err
	}
	err = sev.agent.CheckDeregister(sev.ServiceID)
	if err != nil {
		log.Errorf("[E] deregister check error: %s", err.Error())
	}
	return err
}

// Register TODO
func (sev *Service) Register() error {
	sev.lock.Lock()
	if sev.status&Registered <= 0 {
		sev.status |= Registered
	}
	sev.lock.Unlock()
	// initial register service
	regis := &api.AgentServiceRegistration{
		ID:      sev.ServiceID,
		Name:    sev.ServiceName,
		Address: sev.ServiceIP,
		Port:    sev.ServicePort,
		Tags:    []string{fmt.Sprintf("isleader:%v", sev.leader)},
	}
	err := sev.agent.ServiceRegister(regis)
	if err != nil {
		return err
	}
	// initial register service check
	check := api.AgentServiceCheck{TTL: fmt.Sprintf("%ds", sev.TTL), Status: "passing"}
	err = sev.agent.CheckRegister(&api.AgentCheckRegistration{
		ID:                sev.ServiceID,
		Name:              sev.ServiceName,
		ServiceID:         sev.ServiceID,
		AgentServiceCheck: check,
	})
	return err
}

// UpdateTTL TODO
func (sev *Service) UpdateTTL() error {
	if sev.status&Registered <= 0 {
		return g.ErrNotRegister
	}
	return sev.agent.UpdateTTL(sev.ServiceID, fmt.Sprintf("isleader:%v", sev.leader), "passing")
}
