package cache

import (
	"errors"
	"testing"
	"time"
)

func TestFunc(t *testing.T) {
	c := NewMemory(10, int64(10<<20), -1)
	if val, err := Func(c, "a", func() ([]byte, error) {
		return []byte("b"), nil
	}, time.Millisecond, time.Minute); err != nil || string(val) != "b" {
		t.Error(string(val), err)
	}
	time.Sleep(time.Millisecond * 2)
	if val, err := Func(c, "a", func() ([]byte, error) {
		return nil, errors.New("should absorb error")
	}, time.Millisecond, time.Minute); err != nil || string(val) != "b" {
		t.Error(string(val), err)
	}
	time.Sleep(time.Millisecond * 2)
	if val, err := Func(c, "a", func() ([]byte, error) {
		return []byte("c"), nil
	}, time.Millisecond, time.Minute); err != nil || string(val) != "b" {
		t.Error(string(val), err)
	}
	time.Sleep(time.Millisecond * 2)
	if val, err := Func(c, "a", func() ([]byte, error) {
		return []byte("d"), nil
	}, time.Millisecond*10, time.Minute); err != nil || string(val) != "c" {
		t.Error(string(val), err)
	}
	time.Sleep(time.Millisecond * 2)
	for i := 0; i < 3; i++ {
		if val, err := Func(c, "a", func() ([]byte, error) {
			return []byte("e"), nil
		}, time.Millisecond, time.Minute); err != nil || string(val) != "d" {
			t.Error(string(val), err)
		}
		time.Sleep(time.Millisecond)
	}
}
