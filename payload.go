package cache

import (
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

func newPayload(value []byte) *payload {
	return &payload{
		Value: value,
		V:     v,
	}
}

func (p *payload) WithHeader(header http.Header, status int) *payload {
	p.Header = header
	p.StatusCode = status
	return p
}

func (p *payload) FreshFor(freshFor time.Duration) *payload {
	p.BestBefore = time.Now().Add(freshFor)
	return p
}

func (p payload) NeedRefresh() bool {
	return time.Now().After(p.BestBefore)
}

func (p payload) IsValid() bool {
	return p.V == v
}
