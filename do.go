package cache

import (
	"context"
	"github.com/vmihailenco/msgpack/v5"
	"time"
)

func do(
	ctx context.Context,
	c Cache, key string,
	fn func(context.Context) (*payload, error),
	freshFor, ttl time.Duration,
) (p *payload, err error) {
	if v, err_ := get(c, key); err_ == nil {
		p = v
		if v.NeedRefresh() {
			ctx = DetachContext(ctx)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						// todo log panic
					}
				}()
				if v, err_ := fetch(c, key); err_ == nil {
					if !v.NeedRefresh() {
						return
					}
				}
				_, _ = doCall(ctx, c, key, fn, freshFor, ttl)
			}()
		}
		return
	} else {
		return doCall(ctx, c, key, fn, freshFor, ttl)
	}
}

func doCall(
	ctx context.Context,
	c Cache, key string,
	fn func(context.Context) (*payload, error),
	freshFor, ttl time.Duration,
) (p *payload, err error) {
	// todo singleflight handling
	if p, err = fn(ctx); err != nil {
		if p != nil && err == NoCache {
			err = nil
		}
		return
	}
	if p == nil {
		err = NotFound
		return
	}
	p.FreshFor(freshFor)
	_ = set(c, key, p, ttl)
	return
}

func set(c Cache, key string, p *payload, ttl time.Duration) error {
	if p == nil || !p.IsValid() {
		return nil
	}
	b, err := msgpack.Marshal(p)
	if err != nil {
		return err
	}
	return c.Set(key, b, ttl)
}

func get(c Cache, key string) (p *payload, err error) {
	return parse(c.Get(key))
}

func fetch(c Cache, key string) (p *payload, err error) {
	return parse(c.Fetch(key))
}

func parse(val []byte, e error) (p *payload, err error) {
	if e != nil {
		err = e
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
