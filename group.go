package cache

import (
	"golang.org/x/sync/singleflight"
	"time"
)

type Group struct {
	g singleflight.Group
}

func (g *Group) Race(
	key string, fn func() ([]byte, error), _ time.Duration,
) ([]byte, error) {
	v, err, _ := g.g.Do(key, func() (interface{}, error) {
		return fn()
	})
	g.g.Forget(key)
	if v != nil {
		return v.([]byte), err
	}
	return nil, err
}
