package cache

import (
	"github.com/vmihailenco/msgpack/v5"
	"time"
)

type Payload struct {
	Body       []byte
	Expiration time.Time
}

func GetWithOk(c Cache, key string) (value []byte, ok bool, err error) {
	val, err := c.Get(key)
	if err != nil {
		return
	}
	var v Payload
	if err = msgpack.Unmarshal(val, &v); err != nil {
		return
	}
	return v.Body, !time.Now().After(v.Expiration), nil
}

func SetWithTimeout(c Cache, key string, value []byte, timeout, ttl time.Duration) error {
	b, err := msgpack.Marshal(Payload{
		Body:       value,
		Expiration: time.Now().Add(timeout),
	})
	if err != nil {
		return err
	}
	return c.Set(key, b, ttl)
}
