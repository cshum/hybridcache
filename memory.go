package cache

import (
	"time"

	"github.com/dgraph-io/ristretto"
)

type Memory struct {
	Cache  *ristretto.Cache
	MaxTTL time.Duration
}

func NewMemory(maxItems, maxSize int64, maxTTL time.Duration) *Memory {
	c, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: maxItems * 10,
		MaxCost:     maxSize,
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}
	return &Memory{
		Cache:  c,
		MaxTTL: maxTTL,
	}
}

func (c *Memory) Get(key string, _ bool) ([]byte, error) {
	if res, ok := c.Cache.Get(key); ok {
		return res.([]byte), nil
	}
	return nil, NotFound
}

func (c *Memory) Set(key string, value []byte, ttl time.Duration) error {
	if c.MaxTTL > 0 && ttl > c.MaxTTL {
		ttl = c.MaxTTL
	}
	c.Cache.SetWithTTL(key, value, int64(len(value)), ttl)
	return nil
}
