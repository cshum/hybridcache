package cache

import (
	"context"
	"time"
)

type contextKey struct {
	name string
}

var detachedCtxKey = &contextKey{"Detached"}

type detached struct {
	ctx context.Context
}

func (detached) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (detached) Done() <-chan struct{} {
	return nil
}

func (detached) Err() error {
	return nil
}

func (d detached) Value(key interface{}) interface{} {
	return d.ctx.Value(key)
}

func DetachContext(ctx context.Context) context.Context {
	return context.WithValue(detached{ctx: ctx}, detachedCtxKey, true)
}

func IsContextDetached(ctx context.Context) bool {
	_, ok := ctx.Value(detachedCtxKey).(bool)
	return ok
}
