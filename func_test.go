package cache

import (
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/sync/errgroup"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestFuncDoBytes(t *testing.T) {
	var (
		c    = NewMemory(10, int64(10<<20), -1)
		fn1  = NewFunc(c, time.Millisecond, time.Millisecond, time.Minute)
		fn2  = NewFunc(c, time.Millisecond, time.Millisecond*10, time.Minute)
		fn1j = NewFunc(c, time.Millisecond, time.Millisecond, time.Minute)
	)
	fn1j.Marshal = json.Marshal
	fn1j.Unmarshal = json.Unmarshal
	tests := []struct {
		name         string
		key          string
		c            *Func
		fn           func(ctx context.Context) ([]byte, error)
		wantVal      []byte
		wantErr      string
		wantErrExact error
		noErr        bool
		repeat       int
		sleep        time.Duration
	}{
		{
			name: "should be cache miss",
			key:  "a",
			c:    fn1,
			fn: func(ctx context.Context) ([]byte, error) {
				return []byte("b"), nil
			},
			wantVal: []byte("b"),
			noErr:   true,
			sleep:   time.Millisecond,
		},
		{
			name: "should absorb error",
			key:  "a",
			c:    fn1,
			fn: func(ctx context.Context) ([]byte, error) {
				return nil, errors.New("booommmm")
			},
			wantVal: []byte("b"),
			noErr:   true,
			sleep:   time.Millisecond,
		},
		{
			name: "should use cache",
			key:  "a",
			c:    fn1,
			fn: func(ctx context.Context) ([]byte, error) {
				return []byte("c"), nil
			},
			wantVal: []byte("b"),
			noErr:   true,
			sleep:   time.Millisecond,
		},
		{
			name: "should need refresh",
			key:  "a",
			c:    fn2,
			fn: func(ctx context.Context) ([]byte, error) {
				return []byte("d"), nil
			},
			wantVal: []byte("c"),
			noErr:   true,
			sleep:   time.Millisecond * 2,
		},
		{
			name: "should not need refresh",
			key:  "a",
			c:    fn1,
			fn: func(ctx context.Context) ([]byte, error) {
				return []byte("e"), nil
			},
			wantVal: []byte("d"),
			noErr:   true,
			sleep:   time.Millisecond * 2,
			repeat:  3,
		},
		{
			name: "should return expected error",
			key:  "b",
			c:    fn1,
			fn: func(ctx context.Context) ([]byte, error) {
				return nil, errors.New("expected error")
			},
			wantVal: nil,
			noErr:   false,
			wantErr: "expected error",
			sleep:   time.Millisecond,
		},
		{
			name: "ErrNoCache handling",
			key:  "c",
			c:    fn1,
			fn: func(ctx context.Context) ([]byte, error) {
				if IsDetached(ctx) {
					t.Error("ErrNoCache should not detach")
				}
				return []byte("c1"), ErrNoCache
			},
			wantVal: []byte("c1"),
			noErr:   true,
			sleep:   time.Millisecond,
		},
		{
			name: "ErrNoCache handling",
			key:  "c",
			c:    fn1,
			fn: func(ctx context.Context) ([]byte, error) {
				if IsDetached(ctx) {
					t.Error("ErrNoCache should not detach")
				}
				return []byte("c2"), ErrNoCache
			},
			wantVal: []byte("c2"),
			noErr:   true,
			sleep:   time.Millisecond,
		},
	}
	for _, tt := range tests {
		for i := 0; i <= tt.repeat; i++ {
			t.Run(tt.name, func(t *testing.T) {
				ctx := context.Background()
				val, err := tt.c.DoBytes(ctx, tt.key, tt.fn)
				if tt.noErr && err != nil {
					t.Error(err)
				}
				if !reflect.DeepEqual(val, tt.wantVal) {
					t.Errorf(" = %v, want %v", string(val), string(tt.wantVal))
				}
				if tt.wantErr != "" && (err == nil || err.Error() != tt.wantErr) {
					t.Errorf(" = %v, want %v", err, tt.wantErr)
				}
				if tt.wantErrExact != nil && err != tt.wantErrExact {
					t.Errorf(" = %v, want %v", err, tt.wantErrExact)
				}
				time.Sleep(tt.sleep)
			})
		}
	}
}

func TestFuncDo(t *testing.T) {
	var (
		c    = NewMemory(10, int64(10<<20), -1)
		fn1  = NewFunc(c, time.Millisecond, time.Millisecond, time.Minute)
		fn2  = NewFunc(c, time.Millisecond, time.Millisecond*10, time.Minute)
		fn1j = NewFunc(c, time.Millisecond, time.Millisecond, time.Minute)
	)
	fn1j.Marshal = json.Marshal
	fn1j.Unmarshal = json.Unmarshal
	tests := []struct {
		name         string
		key          string
		c            *Func
		fn           func(ctx context.Context) (interface{}, error)
		wantVal      string
		wantErr      string
		wantErrExact error
		noErr        bool
		repeat       int
		sleep        time.Duration
	}{
		{
			name: "should be cache miss",
			key:  "a",
			c:    fn1,
			fn: func(ctx context.Context) (interface{}, error) {
				return "b", nil
			},
			wantVal: "b",
			noErr:   true,
			sleep:   time.Millisecond,
		},
		{
			name: "should absorb error",
			key:  "a",
			c:    fn1,
			fn: func(ctx context.Context) (interface{}, error) {
				return nil, errors.New("booommmm")
			},
			wantVal: "b",
			noErr:   true,
			sleep:   time.Millisecond,
		},
		{
			name: "should use cache",
			key:  "a",
			c:    fn1,
			fn: func(ctx context.Context) (interface{}, error) {
				return "c", nil
			},
			wantVal: "b",
			noErr:   true,
			sleep:   time.Millisecond,
		},
		{
			name: "should need refresh",
			key:  "a",
			c:    fn2,
			fn: func(ctx context.Context) (interface{}, error) {
				return "d", nil
			},
			wantVal: "c",
			noErr:   true,
			sleep:   time.Millisecond * 2,
		},
		{
			name: "should not need refresh",
			key:  "a",
			c:    fn1,
			fn: func(ctx context.Context) (interface{}, error) {
				return "e", nil
			},
			wantVal: "d",
			noErr:   true,
			repeat:  3,
			sleep:   time.Millisecond * 2,
		},
		{
			name: "cached value corrupted should be treated as cache miss",
			key:  "a",
			c:    fn1j,
			fn: func(ctx context.Context) (interface{}, error) {
				return "asdf", nil
			},
			wantVal: "asdf",
			noErr:   true,
			sleep:   time.Millisecond,
		},
		{
			name: "should return expected error",
			key:  "b",
			c:    fn1,
			fn: func(ctx context.Context) (interface{}, error) {
				return nil, errors.New("expected error")
			},
			noErr:   false,
			wantErr: "expected error",
			sleep:   time.Millisecond,
		},
		{
			name: "ErrNoCache handling",
			key:  "c",
			c:    fn1,
			fn: func(ctx context.Context) (interface{}, error) {
				if IsDetached(ctx) {
					t.Error("ErrNoCache should not detach")
				}
				return "c1", ErrNoCache
			},
			noErr:   true,
			wantVal: "c1",
			sleep:   time.Millisecond,
		},
		{
			name: "ErrNoCache handling",
			key:  "c",
			c:    fn1,
			fn: func(ctx context.Context) (interface{}, error) {
				if IsDetached(ctx) {
					t.Error("ErrNoCache should not detach")
				}
				return "c2", ErrNoCache
			},
			noErr:   true,
			wantVal: "c2",
			sleep:   time.Millisecond,
		},
		{
			name: "should timeout",
			key:  "loooong",
			c:    fn1,
			fn: func(ctx context.Context) (interface{}, error) {
				time.Sleep(time.Second)
				return "dead", nil
			},
			wantVal:      "",
			wantErrExact: context.DeadlineExceeded,
			sleep:        time.Millisecond,
		},
		{
			name: "panic should result error",
			key:  "die",
			c:    fn1,
			fn: func(ctx context.Context) (interface{}, error) {
				panic("booommm")
				return "ok", nil
			},
			wantVal: "",
			wantErr: "booommm",
			sleep:   time.Millisecond,
		},
	}
	for _, tt := range tests {
		for i := 0; i <= tt.repeat; i++ {
			t.Run(tt.name, func(t *testing.T) {
				ctx := context.Background()
				var val string
				err := tt.c.Do(ctx, tt.key, tt.fn, &val)
				if tt.noErr && err != nil {
					t.Error(err)
				}
				if val != tt.wantVal {
					t.Errorf(" = %v, want %v", val, tt.wantVal)
				}
				if tt.wantErr != "" && (err == nil || err.Error() != tt.wantErr) {
					t.Errorf(" = %v, want %v", err, tt.wantErr)
				}
				if tt.wantErrExact != nil && err != tt.wantErrExact {
					t.Errorf(" = %v, want %v", err, tt.wantErrExact)
				}
				time.Sleep(tt.sleep)
			})
		}
	}
}

func TestFunc_Do_Concurrent(t *testing.T) {
	var (
		c1         = NewMemory(10, int64(10<<20), -1)
		fn1        = NewFunc(c1, time.Second, time.Second, time.Minute)
		c2         = NewMemory(10, int64(10<<20), -1)
		fn2        = NewFunc(c2, time.Second, time.Second, time.Minute)
		ctx        = context.Background()
		m          = 10
		n          = 10
		called1    = make(chan int, n*m)
		responded1 = make(chan int, n*m)
		called2    = make(chan int, n*m)
		responded2 = make(chan int, n*m)
	)
	g, ctx := errgroup.WithContext(ctx)
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			(func(j string) {
				g.Go(func() error {
					var val string
					if err := fn1.Do(ctx, j, func(_ context.Context) (interface{}, error) {
						time.Sleep(time.Millisecond * 100)
						called1 <- 1
						return "foo" + j, nil
					}, &val); err != nil || val != "foo"+j {
						t.Error(val, err, "wrong value")
						return nil
					}
					responded1 <- 1
					return nil
				})
				g.Go(func() error {
					var val string
					if err := fn2.Do(ctx, j, func(_ context.Context) (interface{}, error) {
						time.Sleep(time.Millisecond * 100)
						called2 <- 1
						return "bar" + j, nil
					}, &val); err != nil || val != "bar"+j {
						t.Error(val, err, "wrong value")
						return nil
					}
					responded2 <- 1
					return nil
				})
			})(strconv.Itoa(j))
		}
	}
	if err := g.Wait(); err != nil {
		t.Error(err)
	}
	if len(called1) != m || len(called2) != m {
		t.Error(len(called1), len(called2), "should not duplicate concurrent calls")
	}
	if len(responded1) != m*n || len(responded2) != m*n {
		t.Error(len(responded1), len(responded2), "should complete response")
	}
}
