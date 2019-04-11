package consul

import (
	"fmt"
	"time"

	"github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/g"
)

// Leader TODO
type Leader struct {
	service     IService
	lock        *LockEntity
	leader      bool
	session     *SessionEntity
	health      *api.Health
	ServiceName string
	ServiceID   string
	ServiceHost string
	ServicePort int
}

// ILeader TODO
type ILeader interface {
	Deregister() error
	Register() (*ServiceMember, error)
	UpdateTTL() error
	GetServices(passingOnly bool) ([]*ServiceMember, error)
	Select(onLeader func(*ServiceMember))
	Get() (*ServiceMember, error)
	Free()
}

// NewLeader TODO
func NewLeader(
	address string, //127.0.0.1:8500
	lockKey string,
	name string,
	host string,
	port int,
	opts ...ServiceOption,
) ILeader {
	consulConfig := api.DefaultConfig()
	consulConfig.Address = address
	c, err := api.NewClient(consulConfig)
	if err != nil {
		log.Errorf("[E] %s", err.Error())
	}
	session := c.Session()
	kv := c.KV()
	mySession := NewSessionEntity(session, 10)
	sessionID, err := mySession.Create()

	sev := NewService(c.Agent(), name, host, port, opts...)
	l := &Leader{
		service:     sev,
		lock:        NewLockEntity(sessionID, kv, lockKey, 10),
		leader:      false,
		session:     mySession,
		health:      c.Health(),
		ServiceName: name,
		ServiceID:   fmt.Sprintf("%s-%s-%d", name, host, port),
		ServiceHost: host,
		ServicePort: port,
	}
	go func() {
		l.UpdateTTL()
		time.Sleep(time.Second * 2)
	}()
	return l
}

// Deregister deregister service
func (sev *Leader) Deregister() error {
	return sev.service.Deregister()
}

// Register register service
func (sev *Leader) Register() (*ServiceMember, error) {
	err := sev.service.Register()
	leader := &ServiceMember{
		IsLeader:  sev.leader,
		ServiceID: sev.ServiceID,
		Status:    statusOnline,
		ServiceIP: sev.ServiceHost,
		Port:      sev.ServicePort,
	}
	return leader, err
}

// UpdateTTL update service's ttl
func (sev *Leader) UpdateTTL() error {
	return sev.service.UpdateTTL()
}

// GetServices get all service by current service name
func (sev *Leader) GetServices(passingOnly bool) ([]*ServiceMember, error) {
	members, _, err := sev.health.Service(sev.ServiceName, "", passingOnly, nil)
	if err != nil {
		return nil, err
	}
	//return members, err
	data := make([]*ServiceMember, 0)
	for _, v := range members {
		m := &ServiceMember{}
		if v.Checks.AggregatedStatus() == "passing" {
			m.Status = statusOnline
			m.IsLeader = v.Service.Tags[0] == "isleader:true"
		} else {
			m.Status = statusOffline
			m.IsLeader = false
		}
		m.ServiceID = v.Service.ID //Tags[1]
		m.ServiceIP = v.Service.Address
		m.Port = v.Service.Port
		data = append(data, m)
	}
	return data, nil
}

// Select select a leader
func (sev *Leader) Select(onLeader func(*ServiceMember)) {
	go func() {
		leader := &ServiceMember{
			IsLeader:  false,
			ServiceID: sev.ServiceID,
			Status:    statusOnline,
			ServiceIP: sev.ServiceHost,
			Port:      sev.ServicePort,
		}
		success, err := sev.lock.Lock()
		if err == nil {
			sev.leader = success
			sev.service.SetLeader(success)
			leader.IsLeader = success
			go onLeader(leader)
			sev.Register()
		}
		for {
			success, err := sev.lock.Lock()
			if err == nil {
				if success != sev.leader {
					sev.leader = success
					sev.service.SetLeader(success)
					leader.IsLeader = success
					go onLeader(leader)
					sev.Register()
				}
			}
			sev.session.Renew()
			sev.UpdateTTL()
			time.Sleep(time.Second * 3)
		}
	}()
}

// Get get leader service
func (sev *Leader) Get() (*ServiceMember, error) {
	members, _ := sev.GetServices(true)
	if members == nil {
		return nil, g.ErrMembersEmpty
	}
	for _, v := range members {
		if v.IsLeader {
			return v, nil
		}
	}
	return nil, g.ErrLeaderNotFound
}

// Free force free a leader
func (sev *Leader) Free() {
	sev.Deregister()
	if sev.leader {
		sev.lock.Delete()
		sev.leader = false
	}
}
