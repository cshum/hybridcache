package cache

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
)

var errCustomTest = errors.New("custom test error")

func createRedisCache() (c *Redis) {
	c = NewRedis(&redis.Pool{
		Dial: func() (conn redis.Conn, err error) {
			return redis.Dial("tcp", ":6379")
		},
	})
	c.DelayFunc = func(_ int) time.Duration {
		return time.Microsecond
	}
	c.SuppressionTTL = time.Millisecond * 10
	c.Prefix = fmt.Sprintf("!%d!", rand.Int())
	c.ErrorMapper = func(err error) error {
		if err.Error() == "custom test error" {
			return errCustomTest
		}
		return nil
	}
	return
}

func DoTestCache(name string, t *testing.T, c Cache) {
	t.Run(name+"Cache", func(t *testing.T) {
		DoTestCommon(t, c)
		DoTestRace(t, c)
	})
}

func DoTestCommon(t *testing.T, c Cache) {
	t.Run("TestCommon", func(t *testing.T) {
		// not found
		if v, err := c.Get("a"); v != nil || err != ErrNotFound {
			t.Error(err, "should value nil and err not found")
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
	})
}

func DoTestRace(t *testing.T, c Cache) {
	t.Run("TestRace", func(t *testing.T) {
		var (
			m         = 10
			n         = 10
			called    = make(chan int, n*m)
			responded = make(chan int, n*m)
			g, _      = errgroup.WithContext(context.Background())
		)
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				(func(j string) {
					g.Go(func() error {
						b, err := c.Race(j, func() ([]byte, error) {
							called <- 1
							time.Sleep(time.Millisecond * 10)
							if j == "3" {
								return []byte(j), ErrNoCache
							} else if j == "4" {
								return nil, ErrNotFound
							} else if j == "5" {
								return nil, context.DeadlineExceeded
							}
							return []byte(j), nil
						}, time.Second)
						if j == "3" {
							if err != ErrNoCache || string(b) != j {
								t.Error(string(b), err, "error err parsing")
							}
						} else if j == "4" {
							if err != ErrNotFound || b != nil {
								t.Error(string(b), err, "error err parsing")
							}
						} else if j == "5" {
							if err != context.DeadlineExceeded || b != nil {
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
			t.Error(len(called), "should not duplicate concurrent calls")
		}
		if len(responded) != m*n {
			t.Error(len(responded), "should complete response")
		}
	})
}

func TestCache(t *testing.T) {
	DoTestCache("Memory", t, NewMemory(10, int64(10<<20), -1))
	DoTestCache("Redis", t, createRedisCache())
	DoTestCache("Hybrid", t, NewHybrid(
		createRedisCache(),
		NewMemory(10, int64(10<<20), time.Minute*1),
	))
	DoTestCache("HybridRedis", t, NewHybrid(
		createRedisCache(),
		NewMemory(10, int64(10<<20), time.Nanosecond)),
	)
}
