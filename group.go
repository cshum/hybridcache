package cache

import (
	"context"
	"golang.org/x/sync/singleflight"
	"time"
)

type Group struct {
	g singleflight.Group
}

type groupRes struct {
	Res []byte
	Err error
}

func (g *Group) Race(
	key string, fn func() ([]byte, error), waitFor time.Duration,
) ([]byte, error) {
	v, err, _ := g.g.Do(key, func() (interface{}, error) {
		ch := make(chan groupRes, 1)
		go func() {
			b, err := fn()
			ch <- groupRes{b, err}
		}()
		select {
		case res := <-ch:
			return res.Res, res.Err
		case <-time.After(waitFor):
			return nil, context.DeadlineExceeded
		}
	})
	g.g.Forget(key)
	if v != nil {
		return v.([]byte), err
	}
	return nil, err
}
