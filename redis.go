package cache

import (
	"time"

	"github.com/gomodule/redigo/redis"
)

type Redis struct {
	Pool   *redis.Pool
	Prefix string
}

func NewRedis(pool *redis.Pool) *Redis {
	return &Redis{
		Pool: pool,
	}
}

func (r *Redis) Get(key string) (res []byte, err error) {
	var c = r.Pool.Get()
	defer c.Close()
	res, err = redis.Bytes(c.Do("GET", r.Prefix+key))
	if err == redis.ErrNil {
		err = NotFound
	}
	return
}

func (r *Redis) Fetch(key string) ([]byte, error) {
	return r.Get(key)
}

func (r *Redis) Set(key string, value []byte, ttl time.Duration) error {
	var c = r.Pool.Get()
	defer c.Close()
	_, err := c.Do("PSETEX", r.Prefix+key, toMilliseconds(ttl), value)
	return err
}

func toMilliseconds(d time.Duration) int64 {
	return int64(d / time.Millisecond)
}

func fromMilliseconds(pTTL int64) time.Duration {
	return time.Duration(pTTL) * time.Millisecond
}
