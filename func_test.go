package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFunc(t *testing.T) {
	c := NewMemory(10, int64(10<<20), -1)
	ctx := context.Background()
	if val, err := NewFunc(c, time.Millisecond, time.Minute).Do(ctx, "a", func(_ context.Context) ([]byte, error) {
		return []byte("b"), nil
	}); err != nil || string(val) != "b" {
		t.Error(string(val), err)
	}
	time.Sleep(time.Millisecond * 2)
	if val, err := NewFunc(c, time.Millisecond, time.Minute).Do(ctx, "a", func(_ context.Context) ([]byte, error) {
		return nil, errors.New("should absorb error")
	}); err != nil || string(val) != "b" {
		t.Error(string(val), err)
	}
	time.Sleep(time.Millisecond * 2)
	if val, err := NewFunc(c, time.Millisecond, time.Minute).Do(ctx, "a", func(_ context.Context) ([]byte, error) {
		return []byte("c"), nil
	}); err != nil || string(val) != "b" {
		t.Error(string(val), err)
	}
	time.Sleep(time.Millisecond * 2)
	if val, err := NewFunc(c, time.Millisecond*10, time.Minute).Do(ctx, "a", func(_ context.Context) ([]byte, error) {
		return []byte("d"), nil
	}); err != nil || string(val) != "c" {
		t.Error(string(val), err)
	}
	time.Sleep(time.Millisecond * 2)
	for i := 0; i < 3; i++ {
		if val, err := NewFunc(c, time.Millisecond, time.Minute).Do(ctx, "a", func(_ context.Context) ([]byte, error) {
			return []byte("e"), nil
		}); err != nil || string(val) != "d" {
			t.Error(string(val), err)
		}
		time.Sleep(time.Millisecond)
	}
}
