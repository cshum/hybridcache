package cache

import (
	"github.com/vmihailenco/msgpack/v5"
	"net/http"
	"time"
)

const v = 1

type Payload struct {
	_msgpack   struct{} `msgpack:",omitempty"`
	Expiration time.Time
	Value      []byte
	Header     http.Header
	StatusCode int
	V          int
}

func NewPayload(value []byte) *Payload {
	return &Payload{
		Value: value,
		V:     v,
	}
}

func (p *Payload) WithHeader(header http.Header, status int) *Payload {
	p.Header = header
	p.StatusCode = status
	return p
}

func (p *Payload) WithTimeout(timeout time.Duration) *Payload {
	p.Expiration = time.Now().Add(timeout)
	return p
}

func (p Payload) IsExpired() bool {
	return time.Now().After(p.Expiration)
}

func (p Payload) IsValid() bool {
	return p.V == v
}

func SetPayload(c Cache, key string, payload *Payload, ttl time.Duration) error {
	b, err := msgpack.Marshal(payload)
	if err != nil {
		return err
	}
	return c.Set(key, b, ttl)
}

func GetPayload(c Cache, key string) (payload *Payload, err error) {
	val, err := c.Get(key)
	if err != nil {
		return
	}
	payload = &Payload{}
	if err = msgpack.Unmarshal(val, payload); err != nil {
		return
	}
	if !payload.IsValid() {
		err = NotFound
		payload = nil
		return
	}
	return
}
