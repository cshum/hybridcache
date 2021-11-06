package cache

import (
	"time"
)

func Func(
	c Cache, key string,
	fn func() ([]byte, error),
	timeout, ttl time.Duration,
) (value []byte, err error) {
	if v, err_ := GetPayload(c, key); err_ == nil {
		value = v.Value
		if v.IsExpired() {
			go func() {
				if val, err := fn(); err == nil {
					_ = SetPayload(c, key, NewPayload(val).WithTimeout(timeout), ttl)
				}
			}()
		}
		return
	}
	if value, err = fn(); err != nil {
		return
	}
	go func() {
		_ = SetPayload(c, key, NewPayload(value).WithTimeout(timeout), ttl)
	}()
	return
}
