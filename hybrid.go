package cache

import (
	"time"

	"github.com/gomodule/redigo/redis"
)

type Hybrid struct {
	Group
	Pool   *redis.Pool
	Prefix string
	Cache  Cache
}

func NewHybrid(redis *redis.Pool, cache Cache) *Hybrid {
	return &Hybrid{
		Pool:  redis,
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
	var conn = c.Pool.Get()
	defer conn.Close()
	if err = conn.Send("GET", c.Prefix+key); err != nil {
		return
	}
	if err = conn.Send("PTTL", c.Prefix+key); err != nil {
		return
	}
	if err = conn.Flush(); err != nil {
		return
	}
	if value, err = redis.Bytes(conn.Receive()); err != nil {
		if err == redis.ErrNil {
			err = NotFound
		}
		return
	}
	var pTTL int64
	if pTTL, err = redis.Int64(conn.Receive()); err != nil {
		return
	}
	if pTTL > 0 {
		// if redis item has ttl then re-cache
		if err = c.Cache.Set(key, value, fromMilliseconds(pTTL)); err != nil {
			return
		}
	}
	return
}

func (c *Hybrid) Set(key string, value []byte, ttl time.Duration) error {
	if err := c.Cache.Set(key, value, ttl); err != nil {
		return err
	}
	var conn = c.Pool.Get()
	defer conn.Close()
	if _, err := conn.Do("PSETEX", c.Prefix+key, toMilliseconds(ttl), value); err != nil {
		return err
	}
	return nil
}
