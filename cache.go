package cache

import (
	"errors"
	"time"
)

type Cache interface {
	// Get value by key that prioritize quick access over freshness
	Get(key string) ([]byte, error)

	// Fetch the freshest value with its remaining ttl by key
	Fetch(key string) (value []byte, ttl time.Duration, err error)

	// Set value and ttl by key
	Set(key string, value []byte, ttl time.Duration) error

	// Once executes and returns the results of the given function,
	// suppressing multiple calls for a given key under the specified timeout.
	// If a duplicate comes in, the duplicate caller waits for the
	// original to complete and receives the same results.
	Once(key string, fn func() ([]byte, error), timeout time.Duration) ([]byte, error)
}

// ErrNotFound ErrNotFund where result not found
var ErrNotFound = errors.New("hybridcache: not found")

// ErrNoCache denote value should not be cached as an error value,
// which does not result an error
var ErrNoCache = errors.New("hybridcache: no cache")
