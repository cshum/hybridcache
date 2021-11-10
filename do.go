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
	waitFor, freshFor, ttl time.Duration,
) (p *payload, err error) {
	if v, err_ := parse(c.Get(key)); err_ == nil {
		p = v
		if v.NeedRefresh() {
			ctx = DetachContext(ctx)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						// todo log panic
					}
				}()
				if b, _, err_ := c.Fetch(key); err_ == nil {
					if v, err_ := parse(b, nil); err_ == nil {
						if !v.NeedRefresh() {
							return
						}
					}
				}
				_, _ = doCall(ctx, c, key, fn, waitFor, freshFor, ttl)
			}()
		}
		return
	} else {
		return doCall(ctx, c, key, fn, waitFor, freshFor, ttl)
	}
}

func doCall(
	ctx context.Context,
	c Cache, key string,
	fn func(context.Context) (*payload, error),
	waitFor, freshFor, ttl time.Duration,
) (*payload, error) {
	var cancel func()
	if freshFor > 0 {
		ctx, cancel = context.WithTimeout(ctx, freshFor)
		defer cancel()
	}
	return parse(c.Race(key, func() ([]byte, error) {
		p, err := fn(ctx)
		if err != nil {
			if p != nil && err == NoCache {
				return unparse(p)
			}
			return nil, err
		}
		if p == nil {
			return nil, NotFound
		}
		p.FreshFor(freshFor)
		b, err := unparse(p)
		if err != nil {
			return nil, err
		}
		_ = c.Set(key, b, ttl)
		return b, nil
	}, waitFor))
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

func unparse(p *payload) (b []byte, err error) {
	if p == nil || !p.IsValid() {
		err = NotFound
		return
	}
	if b, err = msgpack.Marshal(p); err != nil {
		return
	}
	return
}
