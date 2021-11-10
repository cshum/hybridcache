package cache

import (
	"time"
)

type Hybrid struct {
	Upstream   Cache
	Downstream Cache
}

func NewHybrid(upstream, downstream Cache) *Hybrid {
	return &Hybrid{
		Upstream:   upstream,
		Downstream: downstream,
	}
}

func (c *Hybrid) Get(key string) (value []byte, err error) {
	if val, err_ := c.Downstream.Get(key); err_ == nil {
		value = val
		return
	}
	return c.Fetch(key)
}

func (c *Hybrid) GetWithTTL(key string) (value []byte, ttl time.Duration, err error) {
	return c.Upstream.GetWithTTL(key)
}

func (c *Hybrid) Fetch(key string) (value []byte, err error) {
	var ttl time.Duration
	if value, ttl, err = c.Upstream.GetWithTTL(key); err != nil {
		return
	}
	if ttl > 0 {
		if err = c.Downstream.Set(key, value, ttl); err != nil {
			return
		}
	}
	return
}

func (c *Hybrid) Set(key string, value []byte, ttl time.Duration) error {
	if err := c.Downstream.Set(key, value, ttl); err != nil {
		return err
	}
	return c.Upstream.Set(key, value, ttl)
}

func (c *Hybrid) Race(key string, fn func() ([]byte, error)) ([]byte, error) {
	return c.Downstream.Race(key, func() ([]byte, error) {
		return c.Upstream.Race(key, fn)
	})
}
