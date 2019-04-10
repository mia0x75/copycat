package consul

import (
	"time"

	"github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"
)

// WatchKv TODO
type WatchKv struct {
	kv     *api.KV
	prefix string
	notify []Notify
}

// Notify TODO
type Notify func(kv *api.KV, key string, data interface{})

// WatchKvOption TODO
type WatchKvOption func(k *WatchKv)

// IWatchKv TODO
type IWatchKv interface {
	Watch(watch func([]byte))
}

// NewWatchKv TODO
func NewWatchKv(kv *api.KV, prefix string) IWatchKv {
	k := &WatchKv{
		prefix: prefix,
		kv:     kv,
		notify: make([]Notify, 0),
	}
	return k
}

// Watch TODO
func (m *WatchKv) Watch(watch func([]byte)) {
	go func() {
		lastIndex := uint64(0)
		for {
			_, me, err := m.kv.List(m.prefix, nil)
			if err != nil || me == nil {
				log.Errorf("[E] %s", err.Error())
				time.Sleep(time.Second)
				continue
			}
			lastIndex = me.LastIndex
			break
		}
		for {
			qp := &api.QueryOptions{WaitIndex: lastIndex}
			kp, me, err := m.kv.List(m.prefix, qp)
			if err != nil {
				log.Errorf("[E] %s", err.Error())
				time.Sleep(time.Second)
				continue
			}
			lastIndex = me.LastIndex
			for _, v := range kp {
				if len(v.Value) == 0 {
					continue
				}
				watch(v.Value)
			}
		}
	}()
}
