package consul

import "github.com/hashicorp/consul/api"

// KvEntity TODO
type KvEntity struct {
	kv    IKv
	Key   string
	Value []byte
}

// NewKvEntity TODO
func NewKvEntity(kv *api.KV, key string, value []byte) *KvEntity {
	return &KvEntity{NewKv(kv), key, value}
}

// Set TODO
func (kv *KvEntity) Set() (*KvEntity, error) {
	err := kv.kv.Set(kv.Key, kv.Value)
	return kv, err
}

// Get TODO
func (kv *KvEntity) Get() (*KvEntity, error) {
	v, err := kv.kv.Get(kv.Key)
	kv.Value = v
	return kv, err
}

// Delete TODO
func (kv *KvEntity) Delete() (*KvEntity, error) {
	err := kv.kv.Delete(kv.Key)
	return kv, err
}
