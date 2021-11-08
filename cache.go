package cache

import (
	"context"
	"errors"
	"github.com/vmihailenco/msgpack/v5"
	"time"
)

type Cache interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte, ttl time.Duration) error
}

var NotFound = errors.New("cache: not found")

func do(
	ctx context.Context,
	c Cache, key string,
	fn func(context.Context) (*payload, error),
	timeout, freshFor, ttl time.Duration,
) (p *payload, err error) {
	var cancel = func() {}
	if v, err_ := get(c, key); err_ == nil {
		p = v
		if v.NeedRefresh() {
			ctx = DetachContext(ctx)
			if timeout > 0 {
				ctx, cancel = context.WithTimeout(ctx, timeout)
			}
			go func() {
				defer func() {
					if err := recover(); err != nil {
						// todo log panic
					}
				}()
				defer cancel()
				if v, err := fn(ctx); err == nil {
					if v != nil && v.IsValid() {
						v.FreshFor(freshFor)
						_ = set(c, key, v, ttl)
					}
				}
			}()
		}
		return
	}
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()
	if p, err = fn(ctx); err != nil {
		return
	}
	if p != nil && p.IsValid() {
		p.FreshFor(freshFor)
		go func() {
			_ = set(c, key, p, ttl)
		}()
	}
	return
}

func set(c Cache, key string, p *payload, ttl time.Duration) error {
	b, err := msgpack.Marshal(p)
	if err != nil {
		return err
	}
	return c.Set(key, b, ttl)
}

func get(c Cache, key string) (p *payload, err error) {
	val, err := c.Get(key)
	if err != nil {
		return
	}
	p = &payload{}
	if err = msgpack.Unmarshal(val, p); err != nil {
		return
	}
	if !p.IsValid() {
		err = NotFound
		p = nil
		return
	}
	return
}
