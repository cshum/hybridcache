package cache

import (
	"errors"
	"time"
)

type Cache interface {
	Get(key string) ([]byte, error)
	Fetch(key string) ([]byte, error)
	Set(key string, value []byte, ttl time.Duration) error
}

var NotFound = errors.New("hybrid cache: not found")

var NoCache = errors.New("hybrid cache: no cache")
