package cache

import (
	"time"
)

func Func(
	c Cache, key string,
	fn func() ([]byte, error),
	timeout, ttl time.Duration,
) (value []byte, err error) {
	if val, ok, err_ := GetWithOk(c, key); err_ == nil {
		value = val
		if !ok {
			go func() {
				if val, err := fn(); err == nil {
					_ = SetWithTimeout(c, key, val, timeout, ttl)
				}
			}()
		}
		return
	}
	if value, err = fn(); err != nil {
		return
	}
	go func() {
		_ = SetWithTimeout(c, key, value, timeout, ttl)
	}()
	return
}
