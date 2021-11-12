package cache

import (
	"context"
	"database/sql"
	"github.com/vmihailenco/msgpack/v5"
	"math/rand"
	"time"

	"github.com/gomodule/redigo/redis"
)

type Redis struct {
	// Redigo redis Pool
	Pool *redis.Pool

	// Key Prefix
	Prefix string

	// LockPrefix prefix of lock key, default "!lock!"
	LockPrefix string

	// ErrorMapper maps errors to its corresponding value during unmarshal
	ErrorMapper func(error) error

	// DelayFunc is used to decide the amount of time to wait between lock retries.
	DelayFunc func(tries int) time.Duration

	// SuppressionTTL ttl of value being kept for Once suppression
	// default to 3 seconds. Should be a value larger than maximum DelayFunc
	// but smaller than minimum FreshFor duration
	SuppressionTTL time.Duration

	// SkipLock skips redis lock that manages call suppression for Once method,
	// which result function to be executed immediately.
	// You may want to do so if you would like to skip the extra cost of redis lock.
	SkipLock bool
}

const (
	defaultMinRetryDelayMilliSec = 50
	defaultMaxRetryDelayMilliSec = 250
	defaultSuppressionTTL        = time.Second * 3
	defaultLockPrefix            = "!lock!"
	waitVal                      = "!wait!"
)

type lockRes struct {
	_msgpack struct{} `msgpack:",omitempty"`
	Res      []byte
	Err      error
}

var errMapper = getErrorMapper()

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

func (c *Redis) Once(
	key string, fn func() ([]byte, error), timeout time.Duration,
) (value []byte, err error) {
	if c.SkipLock {
		return fn()
	}
	var (
		resp        []byte
		locked      bool
		retries     int
		e           error
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	)
	defer cancel()
	key = c.lockKey(key)
	for {
		if resp, locked, e = c.lock(key, timeout); e != nil {
			// if redis upstream failed, handle directly
			// instead of crashing downstream consumers
			value, err = fn()
			return
		}
		if locked {
			value, err = fn()
			if e = c.setSuppression(key, value, err); e != nil {
				err = e
			}
			return
		}
		if string(resp) != waitVal {
			value, err = c.parseSuppression(resp)
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
	if err = conn.Send("SET", key, waitVal, "PX", toMilliSec(timeout), "NX"); err != nil {
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

func (c *Redis) setSuppression(key string, value []byte, e error) (err error) {
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

func (c *Redis) parseSuppression(resp []byte) (value []byte, err error) {
	p := &lockRes{}
	if err = msgpack.Unmarshal(resp, p); err != nil {
		return
	}
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
