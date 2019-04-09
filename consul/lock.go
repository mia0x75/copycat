package consul

import (
	"github.com/hashicorp/consul/api"
)

// Lock TODO
type Lock struct {
	sessionID string
	kv        *api.KV
}

// ILock TODO
type ILock interface {
	Lock(key string, timeout int64) (bool, error)
	Unlock(key string) (bool, error)
	Delete(key string) error
}

// NewLock TODO
func NewLock(sessionID string, kv *api.KV) ILock {
	con := &Lock{
		sessionID: sessionID,
		kv:        kv,
	}
	return con
}

// Lock timeout seconds, max lock time, min value is 10 seconds
func (con *Lock) Lock(key string, timeout int64) (bool, error) {
	p := &api.KVPair{Key: key, Value: nil, Session: con.sessionID}
	success, _, err := con.kv.Acquire(p, nil)
	return success, err
}

// Unlock unlock
func (con *Lock) Unlock(key string) (bool, error) {
	p := &api.KVPair{Key: key, Value: nil, Session: con.sessionID}
	success, _, err := con.kv.Release(p, nil)
	return success, err

}

// Delete force unlock
func (con *Lock) Delete(key string) error {
	_, err := con.kv.Delete(key, nil)
	return err
}
