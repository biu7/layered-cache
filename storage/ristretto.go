package storage

import (
	"fmt"
	"time"

	"github.com/dgraph-io/ristretto"
)

var _ Memory = (*Ristretto)(nil)

type Ristretto struct {
	client *ristretto.Cache
}

func NewRistretto(maxMemory int) (*Ristretto, error) {
	if maxMemory <= 0 {
		return nil, fmt.Errorf("ristretto create: invalid maxMemory: %d", maxMemory)
	}

	config := &ristretto.Config{
		NumCounters: int64(maxMemory) * 10,
		MaxCost:     int64(maxMemory),
		BufferItems: 64,
	}

	cache, err := ristretto.NewCache(config)
	if err != nil {
		return nil, fmt.Errorf("ristretto create: maxMemory %d: %w", maxMemory, err)
	}

	return &Ristretto{
		client: cache,
	}, nil
}

func NewRistrettoWithClient(client *ristretto.Cache) (*Ristretto, error) {
	if client == nil {
		return nil, fmt.Errorf("ristretto create: cache is nil")
	}
	return &Ristretto{
		client: client,
	}, nil
}

func (r *Ristretto) Set(key string, value []byte, expire time.Duration) int32 {
	var count int32
	cost := int64(len(key) + len(value))

	ok := r.client.SetWithTTL(key, value, cost, expire)
	if ok {
		count++
		r.client.Wait()
	}
	return count
}

func (r *Ristretto) MSet(values map[string][]byte, expire time.Duration) int32 {
	var count int32
	for key, value := range values {
		cost := int64(len(key) + len(value))
		ok := r.client.SetWithTTL(key, value, cost, expire)
		if ok {
			count++
		}
	}
	r.client.Wait()
	return count
}

func (r *Ristretto) Get(key string) ([]byte, bool) {
	value, found := r.client.Get(key)
	if !found {
		return nil, false
	}

	if byteValue, ok := value.([]byte); ok {
		return byteValue, true
	}
	return nil, false
}

func (r *Ristretto) MGet(keys []string) map[string][]byte {
	ret := make(map[string][]byte)

	for _, key := range keys {
		if value, found := r.client.Get(key); found {
			if byteValue, ok := value.([]byte); ok {
				ret[key] = byteValue
			}
		}
	}
	return ret
}

func (r *Ristretto) Delete(key string) {
	r.client.Del(key)
}
