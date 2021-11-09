package cache

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestFuncDoBytes(t *testing.T) {
	var (
		err  error
		c    = NewMemory(10, int64(10<<20), -1)
		fn1  = NewFunc(c, time.Millisecond, time.Minute)
		fn2  = NewFunc(c, time.Millisecond*10, time.Minute)
		fn1j = NewFunc(c, time.Millisecond, time.Minute)
		ctx  = context.Background()
	)
	fn1j.Marshal = json.Marshal
	fn1j.Unmarshal = json.Unmarshal
	var val []byte
	if val, err = fn1.DoBytes(ctx, "a", func(_ context.Context) ([]byte, error) {
		return []byte("b"), nil
	}); err != nil || string(val) != "b" {
		t.Error(val, err, "should be cache miss")
	}
	time.Sleep(time.Millisecond * 2)
	if val, err = fn1.DoBytes(ctx, "a", func(_ context.Context) ([]byte, error) {
		return nil, errors.New("booommmm")
	}); err != nil || string(val) != "b" {
		t.Error(val, err, "should absorb error")
	}
	time.Sleep(time.Millisecond * 2)
	if val, err = fn1.DoBytes(ctx, "a", func(_ context.Context) ([]byte, error) {
		return []byte("c"), nil
	}); err != nil || string(val) != "b" {
		t.Error(val, err, "should use cache")
	}
	time.Sleep(time.Millisecond * 2)
	if val, err = fn2.DoBytes(ctx, "a", func(_ context.Context) ([]byte, error) {
		return []byte("d"), nil
	}); err != nil || string(val) != "c" {
		t.Error(val, err, "should need refresh")
	}
	time.Sleep(time.Millisecond * 2)
	for i := 0; i < 3; i++ {
		if val, err = fn1.DoBytes(ctx, "a", func(_ context.Context) ([]byte, error) {
			return []byte("e"), nil
		}); err != nil || string(val) != "d" {
			t.Error(val, err, "should not need refresh")
		}
		time.Sleep(time.Millisecond * 2)
	}
	if val, err = fn1.DoBytes(ctx, "b", func(_ context.Context) ([]byte, error) {
		return nil, errors.New("expected error")
	}); err == nil || err.Error() != "expected error" {
		t.Error(val, err, "should return expected error")
	}
	if val, err = fn1.DoBytes(ctx, "c", func(_ context.Context) ([]byte, error) {
		if IsDetached(ctx) {
			t.Error("NoCache should not detech")
		}
		return []byte("c1"), NoCache
	}); err != nil || string(val) != "c1" {
		t.Error(val, err, "NoCache handling")
	}
	if val, err = fn1.DoBytes(ctx, "c", func(_ context.Context) ([]byte, error) {
		if IsDetached(ctx) {
			t.Error("NoCache should not detech")
		}
		return []byte("c2"), NoCache
	}); err != nil || string(val) != "c2" {
		t.Error(val, err, "NoCache handling")
	}
}

func TestFuncDo(t *testing.T) {
	var (
		err  error
		c    = NewMemory(10, int64(10<<20), -1)
		fn1  = NewFunc(c, time.Millisecond, time.Minute)
		fn2  = NewFunc(c, time.Millisecond*10, time.Minute)
		fn1j = NewFunc(c, time.Millisecond, time.Minute)
		ctx  = context.Background()
	)
	fn1j.Marshal = json.Marshal
	fn1j.Unmarshal = json.Unmarshal
	var val string
	if err = fn1.Do(ctx, "a", func(_ context.Context) (interface{}, error) {
		return "b", nil
	}, &val); err != nil || val != "b" {
		t.Error(val, err, "should be cache miss")
	}
	time.Sleep(time.Millisecond * 2)
	if err = fn1.Do(ctx, "a", func(_ context.Context) (interface{}, error) {
		return nil, errors.New("booommmm")
	}, &val); err != nil || val != "b" {
		t.Error(val, err, "should absorb error")
	}
	time.Sleep(time.Millisecond * 2)
	if err = fn1.Do(ctx, "a", func(_ context.Context) (interface{}, error) {
		return "c", nil
	}, &val); err != nil || val != "b" {
		t.Error(val, err, "should use cache")
	}
	time.Sleep(time.Millisecond * 2)
	if err = fn2.Do(ctx, "a", func(_ context.Context) (interface{}, error) {
		return "d", nil
	}, &val); err != nil || val != "c" {
		t.Error(val, err, "should need refresh")
	}
	time.Sleep(time.Millisecond * 2)
	for i := 0; i < 3; i++ {
		if err = fn1.Do(ctx, "a", func(_ context.Context) (interface{}, error) {
			return "e", nil
		}, &val); err != nil || val != "d" {
			t.Error(val, err, "should not need refresh")
		}
		time.Sleep(time.Millisecond * 2)
	}
	if err = fn1j.Do(ctx, "a", func(_ context.Context) (interface{}, error) {
		return "asdf", nil
	}, &val); err != nil || val != "asdf" {
		t.Error(val, err, "cached value corrupted should be treated as cache miss")
	}
	if err = fn1.Do(ctx, "b", func(_ context.Context) (interface{}, error) {
		return nil, errors.New("expected error")
	}, &val); err == nil || err.Error() != "expected error" {
		t.Error(val, err, "should return expected error")
	}
	if err = fn1.Do(ctx, "c", func(_ context.Context) (interface{}, error) {
		if IsDetached(ctx) {
			t.Error("NoCache should not detech")
		}
		return "c1", NoCache
	}, &val); err != nil || val != "c1" {
		t.Error(val, err, "NoCache handling")
	}
	if err = fn1.Do(ctx, "c", func(_ context.Context) (interface{}, error) {
		if IsDetached(ctx) {
			t.Error("NoCache should not detech")
		}
		return "c2", NoCache
	}, &val); err != nil || val != "c2" {
		t.Error(val, err, "NoCache handling")
	}
}
