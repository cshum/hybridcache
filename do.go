package cache

import (
	"context"
	"fmt"
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
	return parse(c.Race(key, func() ([]byte, error) {
		return callWithTimeout(ctx, func(ctx context.Context) ([]byte, error) {
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
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			_ = c.Set(key, b, ttl)
			return b, nil
		}, waitFor)
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

type chanRes struct {
	Res []byte
	Err error
}

func callWithTimeout(
	ctx context.Context,
	fn func(ctx context.Context) ([]byte, error),
	waitFor time.Duration,
) ([]byte, error) {
	var (
		cancel func()
		ch     = make(chan chanRes, 1)
	)
	ctx, cancel = context.WithTimeout(ctx, waitFor)
	defer cancel()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- chanRes{nil, fmt.Errorf("%v", r)}
			}
		}()
		b, err := fn(ctx)
		ch <- chanRes{b, err}
	}()
	select {
	case res := <-ch:
		return res.Res, res.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
