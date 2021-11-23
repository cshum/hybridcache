package cache

import (
	"context"
	"github.com/vmihailenco/msgpack/v5"
	"time"
)

// Func cache client that wraps arbitrary functions
type Func struct {
	// Cache adapter
	Cache Cache

	// WaitFor execution timeout for the function call
	WaitFor time.Duration

	// FreshFor best-before duration of cache before the next refresh
	FreshFor time.Duration

	// TTL duration for cache to stay
	TTL time.Duration

	// custom Marshal function, default msgpack
	Marshal func(interface{}) ([]byte, error)

	// custom Unmarshal function, default msgpack
	Unmarshal func([]byte, interface{}) error

	// AsyncSet calls cache Set in goroutine
	AsyncSet bool
}

// NewFunc creates cache function client with options:
//	waitFor execution timeout,
//	freshFor fresh duration until next refresh,
//	ttl cache time-to-live
func NewFunc(c Cache, waitFor, freshFor, ttl time.Duration) *Func {
	return &Func{
		Cache:    c,
		WaitFor:  waitFor,
		FreshFor: freshFor,
		TTL:      ttl,
	}
}

// Do wraps and returns the result of the given function value pointed to by v.
//
// fn to return error ErrNoCache tells client not to cache the result
// but will not result an error
func (f Func) Do(
	ctx context.Context, key string,
	fn func(context.Context) (interface{}, error),
	v interface{},
) (err error) {
	var pfn = func(ctx context.Context) (*payload, error) {
		v, err := fn(ctx)
		if err != nil && err != ErrNoCache {
			return nil, err
		}
		b, e := f.marshal(v)
		if e != nil {
			return nil, e
		}
		return newPayload(b), err
	}
	var p *payload
	if p, err = do(ctx, f.Cache, key, pfn, f.WaitFor, f.FreshFor, f.TTL, f.AsyncSet); err != nil {
		return
	}
	if err = f.unmarshal(p.Value, v); err != nil {
		// cache payload valid but value corrupted, get live and try once more
		if p, err = doCall(ctx, f.Cache, key, pfn, f.WaitFor, f.FreshFor, f.TTL, f.AsyncSet); err != nil {
			return
		}
		if err = f.unmarshal(p.Value, v); err != nil {
			return
		}
	}
	return
}

// DoBytes wraps and returns the bytes result of the given function.
//
// fn to return error ErrNoCache tells client not to cache the result
// but will not result an error
func (f Func) DoBytes(
	ctx context.Context, key string,
	fn func(context.Context) ([]byte, error),
) (value []byte, err error) {
	var p *payload
	if p, err = do(ctx, f.Cache, key, func(ctx context.Context) (p *payload, err error) {
		var b []byte
		if b, err = fn(ctx); err != nil && err != ErrNoCache {
			return
		}
		p = newPayload(b)
		return
	}, f.WaitFor, f.FreshFor, f.TTL, f.AsyncSet); err != nil {
		return
	}
	value = p.Value
	return
}

func (f Func) marshal(v interface{}) (b []byte, err error) {
	if f.Marshal != nil {
		return f.Marshal(v)
	}
	return msgpack.Marshal(v)
}

func (f Func) unmarshal(b []byte, v interface{}) (err error) {
	if f.Unmarshal != nil {
		return f.Unmarshal(b, v)
	}
	return msgpack.Unmarshal(b, v)
}
