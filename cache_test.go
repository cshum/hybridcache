package cache

import (
	"context"
	"golang.org/x/sync/errgroup"
	"strconv"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
)

func DoTestCache(t *testing.T, c Cache) {
	// not found
	if v, err := c.Get("a"); v != nil || err != NotFound {
		t.Error(err, "should value nil and err not found")
	}
	time.Sleep(time.Millisecond)
	if v, err := c.Get("a"); v != nil || err != NotFound {
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
	if v, err := c.Get("a"); v != nil || err != NotFound {
		t.Error(v, err, "should value nil and err not found")
	}
	DoTestRace(t, c)
}

func DoTestRace(t *testing.T, c Cache) {
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
					if b, err := c.Race(j, func() ([]byte, error) {
						called <- 1
						time.Sleep(time.Millisecond * 50)
						return []byte(j), nil
					}, time.Millisecond*101); err != nil || string(b) != j {
						t.Error(string(b), err, "value should be "+j)
						return nil
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
}

func TestMemory(t *testing.T) {
	DoTestCache(t, NewMemory(10, int64(10<<20), -1))
}

func TestRedis(t *testing.T) {
	DoTestCache(t, NewRedis(&redis.Pool{
		Dial: func() (conn redis.Conn, err error) {
			return redis.Dial("tcp", ":6379")
		},
	}))
}

func TestHybrid(t *testing.T) {
	DoTestCache(t, NewHybrid(
		NewRedis(&redis.Pool{
			Dial: func() (conn redis.Conn, err error) {
				return redis.Dial("tcp", ":6379")
			},
		}),
		NewMemory(10, int64(10<<20), time.Minute*1),
	))
}

func TestHybridRedis(t *testing.T) {
	DoTestCache(t, NewHybrid(
		NewRedis(&redis.Pool{
			Dial: func() (conn redis.Conn, err error) {
				return redis.Dial("tcp", ":6379")
			},
		}),
		NewMemory(10, int64(10<<20), time.Nanosecond)),
	)
}
