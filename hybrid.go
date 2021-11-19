package cache

import (
	"time"
)

// Hybrid cache adaptor based on Upstream and Downstream cache adaptors
type Hybrid struct {
	Upstream   Cache
	Downstream Cache
}

// NewHybrid creates Hybrid cache from upstream and downstream
func NewHybrid(upstream, downstream Cache) *Hybrid {
	return &Hybrid{
		Upstream:   upstream,
		Downstream: downstream,
	}
}

func (c *Hybrid) Get(key string) (value []byte, err error) {
	if val, e := c.Downstream.Get(key); e == nil {
		value = val
		return
	}
	if value, _, err = c.Fetch(key); err != nil {
		return
	}
	return
}

func (c *Hybrid) Fetch(key string) (value []byte, ttl time.Duration, err error) {
	if value, ttl, err = c.Upstream.Fetch(key); err != nil {
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

func (c *Hybrid) Del(keys ...string) error {
	if err := c.Downstream.Del(keys...); err != nil {
		return err
	}
	return c.Upstream.Del(keys...)
}

func (c *Hybrid) Clear() error {
	if err := c.Downstream.Clear(); err != nil {
		return err
	}
	return c.Upstream.Clear()
}

func (c *Hybrid) Close() error {
	if err := c.Downstream.Close(); err != nil {
		return err
	}
	return c.Upstream.Close()
}

func (c *Hybrid) Race(
	key string, fn func() ([]byte, error), timeout time.Duration,
) ([]byte, error) {
	start := time.Now()
	return c.Downstream.Race(key, func() ([]byte, error) {
		return c.Upstream.Race(key, fn, timeout-time.Since(start))
	}, timeout)
}
