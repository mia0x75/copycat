package consul

import (
	"github.com/hashicorp/consul/api"
)

// LockEntity TODO
type LockEntity struct {
	sessionID string
	kv        *api.KV
	key       string
	timeout   int64
	lock      ILock
}

// NewLockEntity TODO
func NewLockEntity(sessionID string, kv *api.KV, key string, timeout int64) *LockEntity {
	lock := NewLock(sessionID, kv)
	return &LockEntity{
		lock:      lock,
		sessionID: sessionID,
		kv:        kv,
		key:       key, timeout: timeout,
	}
}

// Lock timeout seconds, max lock time, min value is 10 seconds
func (con *LockEntity) Lock() (bool, error) {
	return con.lock.Lock(con.key, con.timeout)
}

// Unlock unlock
func (con *LockEntity) Unlock() (bool, error) {
	return con.lock.Unlock(con.key)

}

// Delete force unlock
func (con *LockEntity) Delete() error {
	return con.lock.Delete(con.key)
}
