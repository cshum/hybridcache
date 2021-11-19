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

// Get value by key from downstream, otherwise Fetch from upstream
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

// Fetch from upstream and then sync value by
// Set downstream value the remaining ttl
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

// Set implements the Set method
func (c *Hybrid) Set(key string, value []byte, ttl time.Duration) error {
	if err := c.Downstream.Set(key, value, ttl); err != nil {
		return err
	}
	return c.Upstream.Set(key, value, ttl)
}

// Del implements the Del method
func (c *Hybrid) Del(keys ...string) error {
	if err := c.Downstream.Del(keys...); err != nil {
		return err
	}
	return c.Upstream.Del(keys...)
}

// Clear implements the Clear method
func (c *Hybrid) Clear() error {
	if err := c.Downstream.Clear(); err != nil {
		return err
	}
	return c.Upstream.Clear()
}

// Close implements the Close method
func (c *Hybrid) Close() error {
	if err := c.Downstream.Close(); err != nil {
		return err
	}
	return c.Upstream.Close()
}

// Race implements the Race method by first acquiring downstream and then upstream
func (c *Hybrid) Race(
	key string, fn func() ([]byte, error), timeout time.Duration,
) ([]byte, error) {
	start := time.Now()
	return c.Downstream.Race(key, func() ([]byte, error) {
		return c.Upstream.Race(key, fn, timeout-time.Since(start))
	}, timeout)
}
