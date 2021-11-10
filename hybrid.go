package cache

import (
	"time"

	"github.com/gomodule/redigo/redis"
)

type Hybrid struct {
	Redis
	Cache
}

func NewHybrid(redis *redis.Pool, cache Cache) *Hybrid {
	return &Hybrid{
		Redis: Redis{
			Pool: redis,
		},
		Cache: cache,
	}
}

func (c *Hybrid) Get(key string) (value []byte, err error) {
	if val, err_ := c.Cache.Get(key); err_ == nil {
		value = val
		return
	}
	return c.Fetch(key)
}

func (c *Hybrid) Fetch(key string) (value []byte, err error) {
	var ttl time.Duration
	if value, ttl, err = c.Redis.GetWithTTL(key); err != nil {
		return
	}
	if ttl > 0 {
		if err = c.Cache.Set(key, value, ttl); err != nil {
			return
		}
	}
	return
}

func (c *Hybrid) Set(key string, value []byte, ttl time.Duration) error {
	if err := c.Cache.Set(key, value, ttl); err != nil {
		return err
	}
	return c.Redis.Set(key, value, ttl)
}

func (c *Hybrid) Race(key string, fn func() ([]byte, error)) ([]byte, error) {
	return c.Cache.Race(key, func() ([]byte, error) {
		return c.Redis.Race(key, fn)
	})
}
