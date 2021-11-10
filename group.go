package cache

import "golang.org/x/sync/singleflight"

type Group struct {
	g singleflight.Group
}

func (g *Group) Race(key string, fn func() ([]byte, error)) ([]byte, error) {
	v, err, _ := g.g.Do(key, func() (interface{}, error) {
		return fn()
	})
	g.g.Forget(key)
	return v.([]byte), err
}
