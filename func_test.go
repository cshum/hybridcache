package cache

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestFunc(t *testing.T) {
	c := NewMemory(10, int64(10<<20), -1)
	if val, err := Func(c, "a", func() ([]byte, error) {
		return []byte("b"), nil
	}, time.Nanosecond, time.Minute); err != nil || string(val) != "b" {
		fmt.Println(string(val))
		t.Error(err)
	}
	time.Sleep(time.Millisecond)
	if val, err := Func(c, "a", func() ([]byte, error) {
		return nil, errors.New("should absorb error")
	}, time.Nanosecond, time.Minute); err != nil || string(val) != "b" {
		fmt.Println(string(val))
		t.Error(err)
	}
	time.Sleep(time.Millisecond)
	if val, err := Func(c, "a", func() ([]byte, error) {
		return []byte("c"), nil
	}, time.Nanosecond, time.Minute); err != nil || string(val) != "b" {
		fmt.Println(string(val))
		t.Error(err)
	}
	time.Sleep(time.Millisecond)
	if val, err := Func(c, "a", func() ([]byte, error) {
		return nil, errors.New("should absorb error")
	}, time.Nanosecond, time.Minute); err != nil || string(val) != "c" {
		fmt.Println(string(val))
		t.Error(err)
	}
}
