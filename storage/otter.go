package storage

import (
	"fmt"
	"time"

	"github.com/maypok86/otter"
)

var _ Memory = (*Otter)(nil)

type Otter struct {
	client *otter.CacheWithVariableTTL[string, []byte]
}

func NewOtter(maxMemory int) (*Otter, error) {
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
	return &Otter{
		client: &cache,
	}, nil
}

func NewOtterWithClient(client *otter.CacheWithVariableTTL[string, []byte]) *Otter {
	return &Otter{
		client: client,
	}
}

func (o *Otter) Set(key string, value []byte, expire time.Duration) int32 {
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

func (o *Otter) MSet(values map[string][]byte, expire time.Duration) int32 {
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

func (o *Otter) Get(key string) ([]byte, bool) {
	return o.client.Get(key)
}

func (o *Otter) MGet(keys []string) map[string][]byte {
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

func (o *Otter) Delete(key string) {
	o.client.Delete(key)
}
