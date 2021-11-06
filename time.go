package cache

import (
	"encoding/binary"
	"errors"
	"time"
)

func GetWithOk(c Cache, key string) (value []byte, ok bool, err error) {
	val, err := c.Get(key)
	if err != nil {
		return
	}
	val, ts, err := TimeDecode(val)
	if err != nil {
		return
	}
	return val, !time.Now().After(ts), nil
}

func SetWithTimeout(c Cache, key string, value []byte, timeout, ttl time.Duration) error {
	return c.Set(key, TimeEncode(value, time.Now().Add(timeout)), ttl)
}

func TimeEncode(value []byte, ts time.Time) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(ts.UnixNano()))
	return append(b, value...)
}

func TimeDecode(val []byte) (value []byte, ts time.Time, err error) {
	if len(val) >= 8 {
		value = val[8:]
		// get timestamp byte from value
		ts = time.Unix(0, int64(binary.LittleEndian.Uint64(val[:8])))
	} else {
		err = errors.New("invalid message")
	}
	return
}

func toMilliseconds(d time.Duration) int64 {
	return int64(d / time.Millisecond)
}

func fromMilliseconds(pTTL int64) time.Duration {
	return time.Duration(pTTL) * time.Millisecond
}
