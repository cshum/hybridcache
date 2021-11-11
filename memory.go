package cache

import (
	"golang.org/x/sync/singleflight"
	"time"

	"github.com/dgraph-io/ristretto"
)

type Memory struct {
	g      singleflight.Group
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

func (c *Memory) Get(key string) ([]byte, error) {
	if res, ok := c.Cache.Get(key); ok {
		return res.([]byte), nil
	}
	return nil, NotFound
}

func (c *Memory) Fetch(key string) (value []byte, ttl time.Duration, err error) {
	if value, err = c.Get(key); err != nil {
		return
	}
	ttl, _ = c.Cache.GetTTL(key)
	return
}

func (c *Memory) Set(key string, value []byte, ttl time.Duration) error {
	if c.MaxTTL > 0 && ttl > c.MaxTTL {
		ttl = c.MaxTTL
	}
	c.Cache.SetWithTTL(key, value, int64(len(value)), ttl)
	return nil
}

func (c *Memory) Race(
	key string, fn func() ([]byte, error), _ time.Duration,
) ([]byte, error) {
	v, err, _ := c.g.Do(key, func() (interface{}, error) {
		return fn()
	})
	c.g.Forget(key)
	if v != nil {
		return v.([]byte), err
	}
	return nil, err
}
