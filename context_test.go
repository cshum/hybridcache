package cache

import (
	"context"
	"testing"
	"time"
)

func TestDetachContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	if IsDetached(ctx) {
		t.Error("not detached ctx")
	}
	time.Sleep(time.Millisecond)
	ctx = DetachContext(ctx)
	if err := ctx.Err(); err != nil {
		t.Error(err)
	}
	if !IsDetached(ctx) {
		t.Error("detached ctx")
	}
}