package cache

import (
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
	time.Sleep(time.Second * 1)
	if v, err := c.Get("a"); v != nil || err != NotFound {
		t.Error(v, err, "should value nil and err not found")
	}
	time.Sleep(time.Millisecond)
}

func TestLocal(t *testing.T) {
	DoTestCache(t, NewLocal(10, int64(10<<20), -1))
}

func TestRedis(t *testing.T) {
	DoTestCache(t, NewRedis(&redis.Pool{
		Dial: func() (conn redis.Conn, err error) {
			return redis.Dial("tcp", ":6379")
		},
	}))
}

func TestHybrid(t *testing.T) {
	DoTestCache(t, NewHybrid(&redis.Pool{
		Dial: func() (conn redis.Conn, err error) {
			return redis.Dial("tcp", ":6379")
		},
	}, NewLocal(10, int64(10<<20), time.Minute*1)))
}

func TestHybridRedis(t *testing.T) {
	DoTestCache(t, NewHybrid(&redis.Pool{
		Dial: func() (conn redis.Conn, err error) {
			return redis.Dial("tcp", ":6379")
		},
	}, NewLocal(10, int64(10<<20), time.Nanosecond)))
}
