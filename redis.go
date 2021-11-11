package cache

import (
	"math/rand"
	"time"

	"github.com/gomodule/redigo/redis"
)

const (
	minRetryDelayMilliSec = 50
	maxRetryDelayMilliSec = 250
)

type Redis struct {
	Pool       *redis.Pool
	Prefix     string
	LockPrefix string

	// DelayFunc is used to decide the amount of time to wait between lock retries.
	DelayFunc func(tries int) time.Duration
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

func (c *Redis) Fetch(key string) (value []byte, ttl time.Duration, err error) {
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

func (c *Redis) Set(key string, value []byte, ttl time.Duration) error {
	var conn = c.Pool.Get()
	defer conn.Close()
	if _, err := conn.Do("PSETEX", c.Prefix+key, toMilliseconds(ttl), value); err != nil {
		return err
	}
	return nil
}

func (c *Redis) Race(
	key string, fn func() ([]byte, error), timeout time.Duration,
) ([]byte, error) {
	return fn()
}

func (c *Redis) delayFunc(retries int) time.Duration {
	if c.DelayFunc != nil {
		return c.DelayFunc(retries)
	}
	return time.Duration(rand.Intn(
		maxRetryDelayMilliSec-minRetryDelayMilliSec,
	)+minRetryDelayMilliSec) * time.Millisecond
}

func toMilliseconds(d time.Duration) int64 {
	return int64(d / time.Millisecond)
}

func fromMilliseconds(pTTL int64) time.Duration {
	return time.Duration(pTTL) * time.Millisecond
}
