package cache

import (
	"time"
)

func Func(
	c Cache, key string,
	fn func() ([]byte, error),
	freshFor, ttl time.Duration,
) (value []byte, err error) {
	if v, err_ := getPayload(c, key); err_ == nil {
		value = v.Value
		if v.NeedRefresh() {
			go func() {
				if val, err := fn(); err == nil {
					_ = setPayload(c, key, newPayload(val, freshFor), ttl)
				}
			}()
		}
		return
	}
	if value, err = fn(); err != nil {
		return
	}
	go func() {
		_ = setPayload(c, key, newPayload(value, freshFor), ttl)
	}()
	return
}
