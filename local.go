package cache

import (
	"time"

	"github.com/dgraph-io/ristretto"
)

type Local struct {
	Cache    *ristretto.Cache
	MaxItems int64
	MaxSize  int64
	MaxTTL   time.Duration
}

func NewLocal(maxItems, maxSize int64, maxTTL time.Duration) *Local {
	c, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: maxItems * 10,
		MaxCost:     maxSize,
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}
	return &Local{
		Cache:    c,
		MaxItems: maxItems,
		MaxSize:  maxSize,
		MaxTTL:   maxTTL,
	}
}

func (c *Local) Get(key string) ([]byte, error) {
	if res, ok := c.Cache.Get(key); ok {
		return res.([]byte), nil
	}
	return nil, NotFound
}

func (c *Local) Set(key string, value []byte, ttl time.Duration) error {
	if c.MaxTTL > 0 && ttl > c.MaxTTL {
		ttl = c.MaxTTL
	}
	c.Cache.SetWithTTL(key, value, int64(len(value)), ttl)
	return nil
}
