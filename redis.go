package cache

import (
	"time"

	"github.com/gomodule/redigo/redis"
)

type Redis struct {
	Group  // todo use redis lock for race mechanism
	Pool   *redis.Pool
	Prefix string
}

func NewRedis(pool *redis.Pool) *Redis {
	return &Redis{
		Pool: pool,
	}
}

func (c *Redis) Get(key string) (res []byte, err error) {
	var conn = c.Pool.Get()
	defer conn.Close()
	res, err = redis.Bytes(conn.Do("GET", c.Prefix+key))
	if err == redis.ErrNil {
		err = NotFound
	}
	return
}

func (c *Redis) GetWithTTL(key string) (value []byte, ttl time.Duration, err error) {
	var conn = c.Pool.Get()
	defer conn.Close()
	if err = conn.Send("GET", c.Prefix+key); err != nil {
		return
	}
	if err = conn.Send("PTTL", c.Prefix+key); err != nil {
		return
	}
	if err = conn.Flush(); err != nil {
		return
	}
	if value, err = redis.Bytes(conn.Receive()); err != nil {
		if err == redis.ErrNil {
			err = NotFound
		}
		return
	}
	var pTTL int64
	if pTTL, err = redis.Int64(conn.Receive()); err != nil {
		return
	}
	ttl = fromMilliseconds(pTTL)
	return
}

func (c *Redis) Fetch(key string) ([]byte, error) {
	return c.Get(key)
}

func (c *Redis) Set(key string, value []byte, ttl time.Duration) error {
	var conn = c.Pool.Get()
	defer conn.Close()
	if _, err := conn.Do("PSETEX", c.Prefix+key, toMilliseconds(ttl), value); err != nil {
		return err
	}
	return nil
}

func toMilliseconds(d time.Duration) int64 {
	return int64(d / time.Millisecond)
}

func fromMilliseconds(pTTL int64) time.Duration {
	return time.Duration(pTTL) * time.Millisecond
}
