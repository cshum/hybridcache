package cache

import (
	"context"
	"github.com/vmihailenco/msgpack/v5"
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

func (f Func) DoBytes(
	ctx context.Context, key string,
	fn func(context.Context) ([]byte, error),
) (value []byte, err error) {
	var p *payload
	if p, err = do(ctx, f.Cache, key, func(ctx context.Context) (p *payload, err error) {
		var b []byte
		if b, err = fn(ctx); err != nil {
			return
		}
		p = newPayload(b)
		return
	}, f.Timeout, f.FreshFor, f.TTL); err != nil {
		return
	}
	value = p.Value
	return
}

func (f Func) Do(
	ctx context.Context, key string,
	fn func(context.Context) (interface{}, error),
	v interface{},
) (err error) {
	var p *payload
	if p, err = do(ctx, f.Cache, key, func(ctx context.Context) (p *payload, err error) {
		var (
			v interface{}
			b []byte
		)
		if v, err = fn(ctx); err != nil {
			return
		}
		if f.Marshal != nil {
			if b, err = f.Marshal(v); err != nil {
				return
			}
		} else {
			if b, err = msgpack.Marshal(v); err != nil {
				return
			}
		}
		p = newPayload(b)
		return
	}, f.Timeout, f.FreshFor, f.TTL); err != nil {
		return
	}
	if f.Unmarshal != nil {
		return f.Unmarshal(p.Value, v)
	} else {
		return msgpack.Unmarshal(p.Value, v)
	}
}

func NewFunc(c Cache, freshFor, ttl time.Duration) *Func {
	return &Func{
		Cache:    c,
		FreshFor: freshFor,
		TTL:      ttl,
	}
}
