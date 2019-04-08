package consul

import (
	"errors"

	"github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"
)

var KvDoesNotExists = errors.New("kv does not exists")

type Kv struct {
	kv *api.KV
}
type IKv interface {
	Set(key string, value []byte) error
	Get(key string) ([]byte, error)
	Delete(key string) error
}

func NewKv(kv *api.KV) IKv {
	return &Kv{kv: kv}
}

// set key value
func (k *Kv) Set(key string, value []byte) error {
	log.Debugf("[D] write %s=%s", key, value)
	kv := &api.KVPair{
		Key:   key,
		Value: value,
	}
	_, err := k.kv.Put(kv, nil)
	return err
}

// get key value
// if key does not exists, return error:KvDoesNotExists
func (k *Kv) Get(key string) ([]byte, error) {
	kv, m, e := k.kv.Get(key, nil)
	log.Infof("[I] kv == %+v,", kv)
	log.Infof("[I] m == %+v,", m)
	if e != nil {
		log.Errorf("[E] %+v", e)
		return nil, e
	}
	if kv == nil {
		return nil, KvDoesNotExists
	}
	return kv.Value, nil
}

//delete key value
func (k *Kv) Delete(key string) error {
	_, e := k.kv.Delete(key, nil)
	if e != nil {
		log.Errorf("[E] %+v", e)
	}
	return e
}
