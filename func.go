package cache

import (
	"context"
	"github.com/vmihailenco/msgpack/v5"
	"time"
)

type Func struct {
	Cache    Cache
	WaitFor  time.Duration
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
		if b, err = fn(ctx); err != nil && err != NoCache {
			return
		}
		p = newPayload(b)
		return
	}, f.WaitFor, f.FreshFor, f.TTL); err != nil {
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
	var pfn = func(ctx context.Context) (p *payload, err error) {
		var (
			v interface{}
			b []byte
		)
		if v, err = fn(ctx); err != nil && err != NoCache {
			return
		}
		if b, err = f.marshal(v); err != nil {
			return
		}
		p = newPayload(b)
		return
	}
	if p, err = do(ctx, f.Cache, key, pfn, f.WaitFor, f.FreshFor, f.TTL); err != nil {
		return
	}
	if err = f.unmarshal(p.Value, v); err != nil {
		// cache payload valid but value corrupted, get live and try once more
		if p, err = doCall(ctx, f.Cache, key, pfn, f.WaitFor, f.FreshFor, f.TTL); err != nil {
			return
		}
		if err = f.unmarshal(p.Value, v); err != nil {
			return
		}
	}
	return
}

func (f Func) marshal(v interface{}) (b []byte, err error) {
	if f.Marshal != nil {
		return f.Marshal(v)
	} else {
		return msgpack.Marshal(v)
	}
}

func (f Func) unmarshal(b []byte, v interface{}) (err error) {
	if f.Unmarshal != nil {
		return f.Unmarshal(b, v)
	} else {
		return msgpack.Unmarshal(b, v)
	}
}

func NewFunc(c Cache, waitFor, freshFor, ttl time.Duration) *Func {
	return &Func{
		Cache:    c,
		WaitFor:  waitFor,
		FreshFor: freshFor,
		TTL:      ttl,
	}
}
