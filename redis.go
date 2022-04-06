package cache

import (
	"context"
	"errors"
	"github.com/vmihailenco/msgpack/v5"
	"math/rand"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
)

// Redis cache adaptor based on redigo
type Redis struct {
	// Pool redigo redis pool
	Pool *redis.Pool

	// Prefix of key
	Prefix string

	// LockPrefix prefix of lock key, default "!lock!"
	LockPrefix string

	// DelayFunc is used to decide the amount of time to wait between lock retries.
	DelayFunc func(tries int) time.Duration

	// SkipLock skips redis lock that manages call suppression for Race method,
	// which result function to be executed immediately.
	// This will skip the extra cost of redis lock, if you do not need suppression
	// across multiple servers
	SkipLock bool
}

const (
	defaultMinRetryDelayMilliSec = 50
	defaultMaxRetryDelayMilliSec = 200
	defaultLockPrefix            = "!lock!"
)

type lockRes struct {
	_msgpack struct{} `msgpack:",omitempty"`
	Res      []byte
	Err      error
}

// NewRedis creates redis cache from redigo redis pool
func NewRedis(pool *redis.Pool) *Redis {
	return &Redis{
		Pool: pool,
	}
}

// Get implements the Get method
func (c *Redis) Get(key string) (res []byte, err error) {
	var conn = c.Pool.Get()
	defer conn.Close()
	res, err = redis.Bytes(conn.Do("GET", c.Prefix+key))
	if err == redis.ErrNil {
		err = ErrNotFound
	}
	return
}

// Fetch get value and remaining ttl by key
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
			err = ErrNotFound
		}
		return
	}
	var pTTL int64
	if pTTL, err = redis.Int64(conn.Receive()); err != nil {
		return
	}
	ttl = time.Duration(pTTL) * time.Millisecond
	return
}

// Set implements the Set method
func (c *Redis) Set(key string, value []byte, ttl time.Duration) error {
	var conn = c.Pool.Get()
	defer conn.Close()
	if _, err := conn.Do("PSETEX", c.Prefix+key, ttl.Milliseconds(), value); err != nil {
		return err
	}
	return nil
}

// Del implements the Del method
func (c *Redis) Del(keys ...string) error {
	var keyArgs []interface{}
	for _, key := range keys {
		keyArgs = append(keyArgs, c.Prefix+key)
	}
	if len(keyArgs) > 0 {
		var conn = c.Pool.Get()
		defer conn.Close()
		if _, err := conn.Do("DEL", keyArgs...); err != nil {
			return err
		}
	}
	return nil
}

// Clear implements the Clear method by SCAN keys under prefix and batched DEL
func (c *Redis) Clear() (err error) {
	timeout := time.Minute * 10
	ttl := time.Millisecond * 300
	start := time.Now()
	_, err = c.Race("!clear!", func() (b []byte, err error) {
		err = c.delByPattern(c.Prefix+"*", 5000, timeout)
		return
	}, timeout, ttl)
	// make sure elapsed time > suppression ttl
	if elapsed := time.Since(start); ttl > elapsed {
		time.Sleep(ttl - elapsed)
	}
	return
}

// Close implements the Close method
func (c *Redis) Close() error {
	return c.Pool.Close()
}

// Race implements the Race method using SEX NX
func (c *Redis) Race(
	key string, fn func() ([]byte, error), waitFor, ttl time.Duration,
) (value []byte, err error) {
	if c.SkipLock {
		return fn()
	}
	var (
		retries     int
		lockKey     = c.lockPrefix() + key
		ctx, cancel = context.WithTimeout(context.Background(), waitFor)
	)
	defer cancel()
	for {
		resp, locked, e := c.lock(lockKey, waitFor)
		if e != nil {
			// if redis upstream failed, handle directly
			// instead of crashing downstream consumers
			return fn()
		}
		if locked {
			value, err = fn()
			if e := c.setRaceResp(lockKey, value, err, ttl); e != nil {
				err = e
			}
			return
		} else if len(resp) > 1 || resp[0] != '1' {
			if ok, v, e := c.parseRaceResp(resp); ok {
				value = v
				err = e
				return
			}
		}
		retries++
		delay := c.delayFunc(retries)
		if maxDelay := ttl - defaultMinRetryDelayMilliSec; delay > maxDelay {
			// delay should be within ttl
			delay = maxDelay
		}
		time.Sleep(delay)
		if err = ctx.Err(); err != nil {
			return
		}
	}
}

func (c *Redis) delByPattern(pattern string, n int, timeout time.Duration) (err error) {
	var conn = c.Pool.Get()
	defer conn.Close()
	var (
		iter       = 0
		lockPrefix = c.lockPrefix()
		start      = time.Now()
	)
	for {
		var arr []interface{}
		if arr, err = redis.Values(conn.Do("SCAN", iter, "MATCH", pattern, "COUNT", n)); err != nil {
			return err
		}
		iter, _ = redis.Int(arr[0], nil)
		var keys, _ = redis.Strings(arr[1], nil)
		var delKeys []string
		for _, key := range keys {
			if strings.HasPrefix(key, lockPrefix) {
				continue // should skip delete lock keys
			}
			delKeys = append(delKeys, key)
		}
		if len(delKeys) > 0 {
			if _, err = conn.Do("DEL", redis.Args{}.AddFlat(delKeys)...); err != nil {
				return err
			}
		}
		if iter == 0 {
			break
		}
		if time.Since(start) > timeout {
			return errors.New("timeout")
		}
	}
	return
}

func (c *Redis) lock(
	key string, timeout time.Duration,
) (value []byte, locked bool, err error) {
	var conn = c.Pool.Get()
	defer conn.Close()
	if err = conn.Send("SET", key, "1", "PX", timeout.Milliseconds(), "NX"); err != nil {
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

func (c *Redis) setRaceResp(key string, value []byte, e error, ttl time.Duration) (err error) {
	var (
		p = &lockRes{Res: value, Err: e}
		b []byte
	)
	if b, err = msgpack.Marshal(p); err != nil {
		return
	}
	var conn = c.Pool.Get()
	defer conn.Close()
	if _, err = conn.Do("PSETEX", key, ttl.Milliseconds(), b); err != nil {
		return
	}
	return
}

func (c *Redis) parseRaceResp(resp []byte) (ok bool, value []byte, err error) {
	p := &lockRes{}
	if e := msgpack.Unmarshal(resp, p); e != nil {
		return
	}
	ok = true
	value = p.Res
	err = p.Err
	return
}

func (c *Redis) lockPrefix() string {
	if c.LockPrefix != "" {
		return c.LockPrefix
	}
	return defaultLockPrefix
}

func (c *Redis) delayFunc(retries int) time.Duration {
	if c.DelayFunc != nil {
		return c.DelayFunc(retries)
	}
	return time.Duration(rand.Intn(
		defaultMaxRetryDelayMilliSec-defaultMinRetryDelayMilliSec,
	)+defaultMinRetryDelayMilliSec) * time.Millisecond
}
