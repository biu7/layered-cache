package adapter

import (
	"fmt"
	"time"

	"github.com/maypok86/otter"
)

var _ MemoryAdapter = (*OtterAdapter)(nil)

type OtterAdapter struct {
	client *otter.CacheWithVariableTTL[string, []byte]
}

func NewOtterAdapter(maxMemory int) (*OtterAdapter, error) {
	if maxMemory <= 0 {
		return nil, fmt.Errorf("otter create: invalid maxMemory: %d", maxMemory)
	}
	cache, err := otter.MustBuilder[string, []byte](maxMemory).
		WithVariableTTL().
		Cost(func(key string, value []byte) uint32 {
			return uint32(len(key) + len(value))
		}).
		Build()
	if err != nil {
		return nil, fmt.Errorf("otter create: capacity %d: %w", maxMemory, err)
	}
	return &OtterAdapter{
		client: &cache,
	}, nil
}

func NewOtterAdapterWithClient(client *otter.CacheWithVariableTTL[string, []byte]) (*OtterAdapter, error) {
	if client == nil {
		return nil, fmt.Errorf("otter create: cache is nil")
	}
	return &OtterAdapter{
		client: client,
	}, nil
}

func (o *OtterAdapter) Set(key string, value []byte, expire time.Duration) int32 {
	if expire < 0 {
		expire = 0
	}
	var count int32
	ok := o.client.Set(key, value, expire)
	if ok {
		count++
	}
	return count
}

func (o *OtterAdapter) MSet(values map[string][]byte, expire time.Duration) int32 {
	if expire < 0 {
		expire = 0
	}
	var count int32
	for key, value := range values {
		ok := o.client.Set(key, value, expire)
		if ok {
			count++
		}
	}
	return count
}

func (o *OtterAdapter) Get(key string) ([]byte, bool) {
	return o.client.Get(key)
}

func (o *OtterAdapter) MGet(keys []string) map[string][]byte {
	ret := make(map[string][]byte)
	for _, key := range keys {
		val, success := o.client.Get(key)
		if !success {
			continue
		}
		ret[key] = val
	}
	return ret
}

func (o *OtterAdapter) Delete(key string) {
	o.client.Delete(key)
}
