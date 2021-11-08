package cache

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestFunc(t *testing.T) {
	var (
		err error
		c   = NewMemory(10, int64(10<<20), -1)
		fn1 = NewFunc(c, time.Millisecond, time.Minute)
		fn2 = NewFunc(c, time.Millisecond*10, time.Minute)
		ctx = context.Background()
	)
	var val string
	if err = fn1.Do(ctx, "a", func(_ context.Context) (interface{}, error) {
		return "b", nil
	}, &val); err != nil || val != "b" {
		t.Error(val, err)
	}
	time.Sleep(time.Millisecond * 2)
	if err = fn1.Do(ctx, "a", func(_ context.Context) (interface{}, error) {
		return nil, errors.New("should absorb error")
	}, &val); err != nil || val != "b" {
		t.Error(val, err)
	}
	time.Sleep(time.Millisecond * 2)
	if err = fn1.Do(ctx, "a", func(_ context.Context) (interface{}, error) {
		return "c", nil
	}, &val); err != nil || val != "b" {
		t.Error(val, err)
	}
	time.Sleep(time.Millisecond * 2)
	if err = fn2.Do(ctx, "a", func(_ context.Context) (interface{}, error) {
		return "d", nil
	}, &val); err != nil || val != "c" {
		t.Error(val, err)
	}
	time.Sleep(time.Millisecond * 2)
	for i := 0; i < 3; i++ {
		if err = fn1.Do(ctx, "a", func(_ context.Context) (interface{}, error) {
			return "e", nil
		}, &val); err != nil || val != "d" {
			t.Error(val, err)
		}
		time.Sleep(time.Millisecond)
	}
	// cached value corrupted
	fn1.Marshal = json.Marshal
	fn1.Unmarshal = json.Unmarshal
	if err = fn1.Do(ctx, "a", func(_ context.Context) (interface{}, error) {
		return "asdf", nil
	}, &val); err != nil || val != "asdf" {
		t.Error(val, err)
	}
}
