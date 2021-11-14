package cache

import (
	"context"
	"database/sql"
	"github.com/vmihailenco/msgpack/v5"
	"math/rand"
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

	// ErrorMapper maps errors to its corresponding value during unmarshal
	ErrorMapper func(error) error

	// DelayFunc is used to decide the amount of time to wait between lock retries.
	DelayFunc func(tries int) time.Duration

	// SuppressionTTL ttl of value being kept for Race suppression
	// default to 2 seconds. Should be a value larger than maximum DelayFunc
	// but smaller than minimum FreshFor duration
	SuppressionTTL time.Duration

	// SkipLock skips redis lock that manages call suppression for Race method,
	// which result function to be executed immediately.
	// This will skip the extra cost of redis lock, if you do not need suppression
	// across multiple servers
	SkipLock bool
}

const (
	defaultMinRetryDelayMilliSec = 50
	defaultMaxRetryDelayMilliSec = 250
	defaultSuppressionTTL        = time.Second * 2
	defaultLockPrefix            = "!lock!"
)

type lockRes struct {
	_msgpack struct{} `msgpack:",omitempty"`
	Res      []byte
	Err      error
}

var errMapper = getErrorMapper()

// NewRedis creates redis cache from redigo redis pool
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
		err = ErrNotFound
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
			err = ErrNotFound
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

func (c *Redis) Del(keys ...string) error {
	var keyArgs []interface{}
	for _, key := range keys {
		keyArgs = append(keyArgs, c.Prefix+key)
	}
	var conn = c.Pool.Get()
	defer conn.Close()
	if _, err := conn.Do("DEL", keyArgs...); err != nil {
		return err
	}
	return nil
}

func (c *Redis) Clear() error {
	var conn = c.Pool.Get()
	defer conn.Close()
	if _, err := conn.Do("FLUSHDB"); err != nil {
		return err
	}
	return nil
}

func (c *Redis) Close() error {
	return c.Pool.Close()
}

func (c *Redis) Race(
	key string, fn func() ([]byte, error), timeout time.Duration,
) (value []byte, err error) {
	if c.SkipLock {
		return fn()
	}
	var (
		retries     int
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	)
	defer cancel()
	key = c.lockKey(key)
	for {
		resp, locked, e := c.lock(key, timeout)
		if e != nil {
			// if redis upstream failed, handle directly
			// instead of crashing downstream consumers
			value, err = fn()
			return
		}
		if locked {
			value, err = fn()
			if e := c.setRaceResp(key, value, err); e != nil {
				err = e
			}
			return
		} else if len(resp) > 1 {
			if ok, v, e := c.parseRaceResp(resp); ok {
				value = v
				err = e
				return
			}
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

func (c *Redis) setRaceResp(key string, value []byte, e error) (err error) {
	var (
		p   = &lockRes{Res: value, Err: e}
		b   []byte
		ttl = defaultSuppressionTTL
	)
	if c.SuppressionTTL > 0 {
		ttl = c.SuppressionTTL
	}
	if b, err = msgpack.Marshal(p); err != nil {
		return
	}
	var conn = c.Pool.Get()
	defer conn.Close()
	if _, err = conn.Do("PSETEX", key, toMilliSec(ttl), b); err != nil {
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
	err = c.errorMapper(p.Err)
	return
}

func (c *Redis) lockKey(key string) string {
	if c.LockPrefix != "" {
		return c.LockPrefix + key
	}
	return defaultLockPrefix + key
}

func (c *Redis) delayFunc(retries int) time.Duration {
	if c.DelayFunc != nil {
		return c.DelayFunc(retries)
	}
	return time.Duration(rand.Intn(
		defaultMaxRetryDelayMilliSec-defaultMinRetryDelayMilliSec,
	)+defaultMinRetryDelayMilliSec) * time.Millisecond
}

func toMilliSec(d time.Duration) int64 {
	return int64(d / time.Millisecond)
}

func fromMilliSec(pTTL int64) time.Duration {
	return time.Duration(pTTL) * time.Millisecond
}

func (c *Redis) errorMapper(err error) error {
	if err == nil {
		return nil
	}
	if c.ErrorMapper != nil {
		if e := c.ErrorMapper(err); e != err && e != nil {
			return e
		}
	}
	if e, ok := errMapper[err.Error()]; ok {
		return e
	}
	return err
}

func getErrorMapper() map[string]error {
	// map common errors to their original
	errMapper := map[string]error{}
	for _, err := range []error{
		ErrNotFound,
		ErrNoCache,
		context.Canceled,
		context.DeadlineExceeded,
		sql.ErrNoRows,
		sql.ErrTxDone,
		sql.ErrConnDone,
		redis.ErrNil,
		redis.ErrPoolExhausted,
	} {
		errMapper[err.Error()] = err
	}
	return errMapper
}
