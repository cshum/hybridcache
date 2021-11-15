package cache

import (
	"errors"
	"time"
)

// Cache interface for cache adaptor
type Cache interface {
	// Get value by key that prioritize quick access over freshness
	Get(key string) (value []byte, err error)

	// Fetch the freshest value with its remaining ttl by key
	Fetch(key string) (value []byte, ttl time.Duration, err error)

	// Set value and ttl by key
	Set(key string, value []byte, ttl time.Duration) error

	// Del deletes items from the cache by keys
	Del(keys ...string) error

	// Clear the cache
	Clear() error

	// Close releases the resources used by the cache
	Close() error

	// Race executes and returns the results and error of the given function once
	// under specified timeout, suppressing multiple calls of the same key.
	// If a duplicate comes in, the duplicate caller waits for the
	// original to complete and receives the same results.
	Race(key string, fn func() ([]byte, error), timeout time.Duration) ([]byte, error)
}

// ErrNotFound result not found
var ErrNotFound = errors.New("hybridcache: not found")

// ErrNoCache denotes result should not be cached,
// does not result an error to the endpoint
var ErrNoCache = errors.New("hybridcache: no cache")
