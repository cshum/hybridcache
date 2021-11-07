package cache

import (
	"github.com/vmihailenco/msgpack/v5"
	"net/http"
	"time"
)

const v = 1

type payload struct {
	_msgpack   struct{} `msgpack:",omitempty"`
	BestBefore time.Time
	Value      []byte
	Header     http.Header
	StatusCode int
	V          int
}

func newPayload(value []byte, freshFor time.Duration) *payload {
	return &payload{
		Value:      value,
		BestBefore: time.Now().Add(freshFor),
		V:          v,
	}
}

func (p *payload) WithHeader(header http.Header, status int) *payload {
	p.Header = header
	p.StatusCode = status
	return p
}

func (p payload) NeedRefresh() bool {
	return time.Now().After(p.BestBefore)
}

func (p payload) IsValid() bool {
	return p.V == v
}

func setPayload(c Cache, key string, p *payload, ttl time.Duration) error {
	b, err := msgpack.Marshal(p)
	if err != nil {
		return err
	}
	return c.Set(key, b, ttl)
}

func getPayload(c Cache, key string) (p *payload, err error) {
	val, err := c.Get(key)
	if err != nil {
		return
	}
	p = &payload{}
	if err = msgpack.Unmarshal(val, p); err != nil {
		return
	}
	if !p.IsValid() {
		err = NotFound
		p = nil
		return
	}
	return
}
