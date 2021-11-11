package cache

import (
	"context"
	"errors"
	"github.com/vmihailenco/msgpack/v5"
	"math/rand"
	"time"

	"github.com/gomodule/redigo/redis"
)

const (
	minRetryDelayMilliSec = 50
	maxRetryDelayMilliSec = 250
)

type lockRes struct {
	Res []byte
	Err string
}

type Redis struct {
	Pool *redis.Pool

	Prefix string

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
	ttl = fromMilliSec(pTTL)
	return
}

func (c *Redis) Set(key string, value []byte, ttl time.Duration) error {
	var conn = c.Pool.Get()
	defer conn.Close()
	if _, err := conn.Do("PSETEX", c.Prefix+key, toMilliSec(ttl), value); err != nil {
		return err
	}
	return nil
}

func (c *Redis) Race(
	key string, fn func() ([]byte, error), timeout time.Duration,
) (value []byte, err error) {
	var (
		resp        []byte
		locked      bool
		retries     int
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	)
	defer cancel()
	key = c.lockKey(key)
	for {
		if resp, locked, err = c.lock(key, timeout); err != nil {
			return
		}
		if locked {
			value, err = fn()
			if e := c.setLockValue(key, value, err, timeout); e != nil {
				err = e
			}
			return
		}
		if len(resp) > 1 {
			value, err = unparseLockValue(resp)
			return
		}
		time.Sleep(c.delayFunc(retries))
		retries++
		if err = ctx.Err(); err != nil {
			return
		}
	}
}

func (c *Redis) lock(
	key string, timeout time.Duration,
) (value []byte, locked bool, err error) {
	var conn = c.Pool.Get()
	defer conn.Close()
	if err = conn.Send("SET", key, "1", "PX", toMilliSec(timeout), "NX"); err != nil {
		return
	}
	if err = conn.Send("GET", key); err != nil {
		return
	}
	if err = conn.Flush(); err != nil {
		return
	}
	if ok, e := redis.String(conn.Receive()); ok == "OK" && e == nil {
		locked = true
	}
	if value, err = redis.Bytes(conn.Receive()); err != nil {
		return
	}
	return
}

func (c *Redis) setLockValue(
	key string, value []byte, e error, ttl time.Duration,
) (err error) {
	var (
		p = &lockRes{Res: value}
		b []byte
	)
	if e != nil {
		p.Err = e.Error()
	}
	if b, err = msgpack.Marshal(p); err != nil {
		return
	}
	if err = c.Set(key, b, ttl); err != nil {
		return
	}
	return
}

func unparseLockValue(resp []byte) (value []byte, err error) {
	p := &lockRes{}
	if err = msgpack.Unmarshal(resp, p); err != nil {
		return
	}
	value = p.Res
	if p.Err != "" {
		err = errors.New(p.Err)
	}
	return
}

func (c *Redis) lockKey(key string) string {
	if c.LockPrefix != "" {
		return c.LockPrefix + key
	}
	return "!lock!" + key
}

func (c *Redis) delayFunc(retries int) time.Duration {
	if c.DelayFunc != nil {
		return c.DelayFunc(retries)
	}
	return time.Duration(rand.Intn(
		maxRetryDelayMilliSec-minRetryDelayMilliSec,
	)+minRetryDelayMilliSec) * time.Millisecond
}

func toMilliSec(d time.Duration) int64 {
	return int64(d / time.Millisecond)
}

func fromMilliSec(pTTL int64) time.Duration {
	return time.Duration(pTTL) * time.Millisecond
}
