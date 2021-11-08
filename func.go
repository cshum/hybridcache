package cache

import (
	"context"
	"encoding/json"
	"time"
)

type Func struct {
	Cache    Cache
	Timeout  time.Duration
	FreshFor time.Duration
	TTL      time.Duration

	Marshal   func(interface{}) ([]byte, error)
	Unmarshal func([]byte, interface{}) error
}

func (f Func) do(
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
				defer func() {
					if err := recover(); err != nil {
						// todo log panic
					}
				}()
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

func (f Func) Do(
	ctx context.Context, key string,
	fn func(context.Context) (interface{}, error),
	v interface{},
) (err error) {
	var b []byte
	if b, err = f.do(ctx, key, func(ctx context.Context) (val []byte, err error) {
		var v interface{}
		if v, err = fn(ctx); err != nil {
			return
		}
		if f.Marshal != nil {
			return f.Marshal(v)
		} else {
			return json.Marshal(v)
		}
	}); err != nil {
		return
	}
	if f.Unmarshal != nil {
		return f.Unmarshal(b, v)
	} else {
		return json.Unmarshal(b, v)
	}
}

func NewFunc(c Cache, freshFor, ttl time.Duration) *Func {
	return &Func{
		Cache:    c,
		FreshFor: freshFor,
		TTL:      ttl,

		Marshal:   json.Marshal,
		Unmarshal: json.Unmarshal,
	}
}
