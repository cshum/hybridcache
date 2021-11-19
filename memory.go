package cache

import (
	"golang.org/x/sync/singleflight"
	"time"

	"github.com/dgraph-io/ristretto"
)

// Memory cache adaptor based on ristretto
type Memory struct {
	g singleflight.Group

	// Cache ristretto in-memory cache
	Cache *ristretto.Cache

	// MaxTTL bounded maximum ttl
	MaxTTL time.Duration
}

// NewMemory creates an in-memory cache with an upper bound for
// maxItems total number of items, maxSize total byte size
// maxTTL max ttl of each item
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

// Get implements the Get method
func (c *Memory) Get(key string) ([]byte, error) {
	if res, ok := c.Cache.Get(key); ok {
		return res.([]byte), nil
	}
	return nil, ErrNotFound
}

// Fetch get value and remaining ttl by key
func (c *Memory) Fetch(key string) (value []byte, ttl time.Duration, err error) {
	if value, err = c.Get(key); err != nil {
		return
	}
	ttl, _ = c.Cache.GetTTL(key)
	return
}

// Set implements the Set method
func (c *Memory) Set(key string, value []byte, ttl time.Duration) error {
	if c.MaxTTL > 0 && ttl > c.MaxTTL {
		ttl = c.MaxTTL
	}
	c.Cache.SetWithTTL(key, value, int64(len(value)), ttl)
	return nil
}

// Del implements the Del method
func (c *Memory) Del(keys ...string) error {
	for _, key := range keys {
		c.Cache.Del(key)
	}
	return nil
}

// Clear implements the Clear method
func (c *Memory) Clear() error {
	c.Cache.Clear()
	return nil
}

// Close implements the Close method
func (c *Memory) Close() error {
	c.Cache.Close()
	return nil
}

// Race implements the Race method using singleflight
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
