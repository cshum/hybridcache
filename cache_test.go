package cache

import (
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
)

var redisCnt int

func createRedisCache() (c *Redis) {
	c = NewRedis(&redis.Pool{
		Dial: func() (conn redis.Conn, err error) {
			return redis.Dial("tcp", ":6379")
		},
	})
	c.DelayFunc = func(_ int) time.Duration {
		return time.Microsecond
	}
	if redisCnt > 0 {
		c.Prefix = fmt.Sprintf("!%d!", rand.Int())
	}
	redisCnt++
	return
}

func TestCache_Common(t *testing.T) {
	DoTestCacheCommon("Memory", t, NewMemory(10, int64(10<<20), -1))
	DoTestCacheCommon("HybridMemory", t, NewHybrid(
		NewMemory(10, int64(10<<20), time.Minute*1),
		NewMemory(10, int64(10<<20), time.Minute*1),
	))
	DoTestCacheCommon("Redis", t, createRedisCache())
	DoTestCacheCommon("HybridRedis", t, NewHybrid(
		createRedisCache(),
		NewMemory(10, int64(10<<20), time.Minute*1),
	))
}

func TestCache_Race(t *testing.T) {
	DoTestCacheRace(
		"Memory", t, NewMemory(10, int64(10<<20), -1),
		10, 10, time.Millisecond*100)
	DoTestCacheRace("HybridMemory", t, NewHybrid(
		NewMemory(10, int64(10<<20), time.Minute*1),
		NewMemory(10, int64(10<<20), time.Minute*1),
	), 10, 10, time.Millisecond*100)
	DoTestCacheRace(
		"Redis", t, createRedisCache(), 5, 5, time.Millisecond*300)
	time.Sleep(time.Millisecond * 10)
	DoTestCacheRace("HybridRedis", t, NewHybrid(
		createRedisCache(),
		NewMemory(10, int64(10<<20), time.Minute*1),
	), 5, 5, time.Millisecond*300)
	time.Sleep(time.Millisecond * 10)
}

func DoTestCacheCommon(name string, t *testing.T, c Cache) {
	t.Run(name+"TestCommon", func(t *testing.T) {
		// not found
		if v, err := c.Get("a"); v != nil || err != ErrNotFound {
			t.Error(err, "should value nil and err not found")
		}
		time.Sleep(time.Millisecond)
		if err := c.Set("a", []byte("abc"), time.Second*2046); err != nil {
			t.Error(err)
		}
		time.Sleep(time.Millisecond)
		if v, err := c.Get("a"); string(v) != "abc" || err == ErrNotFound {
			t.Error(err, "should value and no error")
		}
		if err := c.Del("a"); err != nil {
			t.Error(err, "should del")
		}
		time.Sleep(time.Millisecond)
		if v, err := c.Get("a"); v != nil || err != ErrNotFound {
			t.Error(err, "should value nil and err not found")
		}
		time.Sleep(time.Millisecond)
		// set and found
		if err := c.Set("a", []byte{'b'}, time.Millisecond*100); err != nil {
			t.Error(err)
		}
		time.Sleep(time.Millisecond)
		if v, err := c.Get("a"); string(v) != "b" || err != nil {
			t.Error(err, "should value and no error")
		}
		time.Sleep(time.Millisecond)
		if v, err := c.Get("a"); string(v) != "b" || err != nil {
			t.Error(err, "should value and no error")
		}
		time.Sleep(time.Millisecond * 100)
		if v, err := c.Get("a"); v != nil || err != ErrNotFound {
			t.Error(v, err, "should value nil and err not found")
		}
		time.Sleep(time.Millisecond)
		if err := c.Set("a", []byte("abc"), time.Second*2046); err != nil {
			t.Error(err)
		}
		time.Sleep(time.Millisecond)
		if err := c.Clear(); err != nil {
			t.Error(err, "should clear")
		}
		time.Sleep(time.Millisecond)
		if v, err := c.Get("a"); v != nil || err != ErrNotFound {
			t.Error(err, "should value nil and err not found")
		}
		time.Sleep(time.Millisecond)
		if err := c.Close(); err != nil {
			t.Error(err, "error closing cache")
		}
	})
}

func DoTestCacheRace(name string, t *testing.T, c Cache, m, n int, sleep time.Duration) {
	t.Run(name+"TestRace", func(t *testing.T) {
		var (
			called    = make(chan int, n*m)
			responded = make(chan int, n*m)
			g, _      = errgroup.WithContext(context.Background())
		)
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				(func(j string) {
					g.Go(func() error {
						b, err := c.Race(j, func() ([]byte, error) {
							time.Sleep(sleep)
							called <- 1
							if j == "2" {
								return nil, nil
							} else if j == "3" {
								return []byte(j), ErrNoCache
							} else if j == "4" {
								return nil, ErrNotFound
							} else if j == "5" {
								return nil, context.DeadlineExceeded
							}
							return []byte(j), nil
						}, time.Second*5, time.Millisecond*10)
						if j == "2" {
							if err != nil || b != nil {
								t.Error(b, err, "expected nil")
							}
						} else if j == "3" {
							if err == nil || err.Error() != ErrNoCache.Error() || string(b) != j {
								t.Error(string(b), err, "error err parsing")
							}
						} else if j == "4" {
							if err == nil || err.Error() != ErrNotFound.Error() || b != nil {
								t.Error(string(b), err, "error err parsing")
							}
						} else if j == "5" {
							if err == nil || err.Error() != context.DeadlineExceeded.Error() || b != nil {
								t.Error(string(b), err, "error err parsing")
							}
						} else {
							if err != nil || string(b) != j {
								t.Error(string(b), err, "value should be "+j)
							}
						}
						responded <- 1
						return nil
					})
				})(strconv.Itoa(j))
			}
		}
		if err := g.Wait(); err != nil {
			t.Error(err)
		}
		if len(called) != m {
			t.Errorf(" = %v, want %v", len(called), m)
		}
		if len(responded) != m*n {
			t.Errorf(" = %v, want %v", len(responded), m*n)
		}
		if err := c.Close(); err != nil {
			t.Error(err, "error closing cache")
		}
	})
}
