package storage

import (
	"fmt"
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

var _ Memory = (*Ristretto)(nil)

type Ristretto struct {
	client *ristretto.Cache[string, []byte]
}

func NewRistretto(maxMemory int) (*Ristretto, error) {
	if maxMemory <= 0 {
		return nil, fmt.Errorf("ristretto create: invalid maxMemory: %d", maxMemory)
	}

	// If you need to customize the Config, please use NewRistrettoWithClient instead.
	config := &ristretto.Config[string, []byte]{
		NumCounters: 1e7,
		MaxCost:     int64(maxMemory),
		BufferItems: 64,
	}

	cache, err := ristretto.NewCache[string, []byte](config)
	if err != nil {
		return nil, fmt.Errorf("ristretto create: maxMemory %d: %w", maxMemory, err)
	}

	return &Ristretto{
		client: cache,
	}, nil
}

func NewRistrettoWithClient(client *ristretto.Cache[string, []byte]) *Ristretto {
	return &Ristretto{
		client: client,
	}
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

	return value, true
}

func (r *Ristretto) MGet(keys []string) map[string][]byte {
	ret := make(map[string][]byte)

	for _, key := range keys {
		if value, found := r.client.Get(key); found {
			ret[key] = value
		}
	}
	return ret
}

func (r *Ristretto) Delete(key string) {
	r.client.Del(key)
}
