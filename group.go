package cache

import "golang.org/x/sync/singleflight"

type Group struct {
	g singleflight.Group
}

func (r *Group) Race(
	key string, fn func() (interface{}, error),
) (v interface{}, err error) {
	v, err, _ = r.g.Do(key, fn)
	r.g.Forget(key)
	return
}
