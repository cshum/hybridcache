package cache

import (
	"context"
	"time"
)

type Func struct {
	Cache    Cache
	Timeout  time.Duration
	FreshFor time.Duration
	TTL      time.Duration
}

func (f *Func) Do(
	ctx context.Context, key string,
	fn func(context.Context) ([]byte, error),
) (value []byte, err error) {
	var cancel = func() {}
	if v, err_ := getPayload(f.Cache, key); err_ == nil {
		value = v.Value
		if v.NeedRefresh() {
			ctx = DetachContext(ctx)
			if f.Timeout > 0 {
				ctx, cancel = context.WithTimeout(ctx, f.Timeout)
			}
			go func() {
				defer cancel()
				if val, err := fn(ctx); err == nil {
					_ = setPayload(f.Cache, key, newPayload(val, f.FreshFor), f.TTL)
				}
			}()
		}
		return
	}
	if f.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, f.Timeout)
	}
	defer cancel()
	if value, err = fn(ctx); err != nil {
		return
	}
	go func() {
		_ = setPayload(f.Cache, key, newPayload(value, f.FreshFor), f.TTL)
	}()
	return
}

func NewFunc(c Cache, freshFor, ttl time.Duration) *Func {
	return &Func{
		Cache:    c,
		FreshFor: freshFor,
		TTL:      ttl,
	}
}
