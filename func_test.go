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

func TestFunc_Do(t *testing.T) {
	DoTestFuncDo("Memory", t, NewMemory(10, int64(10<<20), -1))
	DoTestFuncDo("Redis", t, createRedisCache())
	DoTestFuncDo("Hybrid", t, NewHybrid(
		createRedisCache(),
		NewMemory(10, int64(10<<20), -1),
	))
}

func TestFunc_DoBytes(t *testing.T) {
	DoTestFuncDoBytes("Memory", t, NewMemory(10, int64(10<<20), -1))
	DoTestFuncDoBytes("Redis", t, createRedisCache())
	time.Sleep(time.Millisecond * 10)
	DoTestFuncDoBytes("Hybrid", t, NewHybrid(
		createRedisCache(),
		NewMemory(10, int64(10<<20), -1),
	))
	time.Sleep(time.Millisecond * 10)
}

func TestFunc_Do_Concurrent(t *testing.T) {
	DoTestFuncDoConcurrent(
		"Memory", t, NewMemory(10, int64(10<<20), -1),
		10, 10, time.Millisecond*100)
	DoTestFuncDoConcurrent(
		"Redis", t, createRedisCache(), 5, 5, time.Millisecond*300)
	DoTestFuncDoConcurrent("Hybrid", t, NewHybrid(
		createRedisCache(),
		NewMemory(10, int64(10<<20), -1),
	), 10, 10, time.Millisecond*100)
}

func DoTestFuncDoBytes(name string, t *testing.T, c Cache) {
	t.Run(name+"FuncDo", func(t *testing.T) {
		var (
			fn1  = NewFunc(c, time.Millisecond*50, time.Millisecond*50, time.Second*2)
			fn1j = NewFunc(c, time.Millisecond*50, time.Millisecond*50, time.Second*2)
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
				sleep:   time.Millisecond * 101,
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
				sleep:   time.Millisecond * 101,
			},
			{
				name: "should need refresh",
				key:  "a",
				c:    fn1,
				fn: func(ctx context.Context) ([]byte, error) {
					return []byte("d"), nil
				},
				wantVal: []byte("c"),
				noErr:   true,
				sleep:   time.Millisecond * 2,
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
				sleep:   time.Millisecond * 100,
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
			{
				name: "should timeout",
				key:  "loooong",
				c:    fn1,
				fn: func(ctx context.Context) ([]byte, error) {
					time.Sleep(time.Second)
					return []byte("dead"), nil
				},
				wantVal:      nil,
				wantErrExact: context.DeadlineExceeded,
				sleep:        time.Millisecond,
			},
			{
				name: "panic should result error",
				key:  "die",
				c:    fn1,
				fn: func(ctx context.Context) ([]byte, error) {
					panic("booommm")
					return []byte("ok"), nil
				},
				wantVal: nil,
				wantErr: "booommm",
				sleep:   time.Millisecond,
			},
			{
				name: "should return same custom error",
				key:  "err",
				c:    fn1,
				fn: func(ctx context.Context) ([]byte, error) {
					return []byte("abc"), errCustomTest
				},
				wantVal:      nil,
				wantErrExact: errCustomTest,
				sleep:        time.Millisecond,
			},
		}
		for _, tt := range tests {
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
	})
}

func DoTestFuncDo(name string, t *testing.T, c Cache) {
	t.Run(name+"FuncDo", func(t *testing.T) {
		var (
			fn     = NewFunc(c, time.Millisecond*50, time.Millisecond*50, time.Second*2)
			fnJson = NewFunc(c, time.Millisecond*50, time.Millisecond*50, time.Second*2)
		)
		fnJson.Marshal = json.Marshal
		fnJson.Unmarshal = json.Unmarshal
		tests := []struct {
			name         string
			key          string
			c            *Func
			fn           func(ctx context.Context) (interface{}, error)
			wantVal      string
			wantErr      string
			wantErrExact error
			noErr        bool
			sleep        time.Duration
		}{
			{
				name: "should be cache miss",
				key:  "a",
				c:    fn,
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
				c:    fn,
				fn: func(ctx context.Context) (interface{}, error) {
					return nil, errors.New("booommmm")
				},
				wantVal: "b",
				noErr:   true,
				sleep:   time.Millisecond * 101,
			},
			{
				name: "should use cache",
				key:  "a",
				c:    fn,
				fn: func(ctx context.Context) (interface{}, error) {
					return "c", nil
				},
				wantVal: "b",
				noErr:   true,
				sleep:   time.Millisecond * 101,
			},
			{
				name: "should need refresh",
				key:  "a",
				c:    fn,
				fn: func(ctx context.Context) (interface{}, error) {
					return "d", nil
				},
				wantVal: "c",
				noErr:   true,
				sleep:   time.Millisecond * 101,
			},
			{
				name: "cached value corrupted should be treated as cache miss",
				key:  "a",
				c:    fnJson,
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
				c:    fn,
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
				c:    fn,
				fn: func(ctx context.Context) (interface{}, error) {
					if IsDetached(ctx) {
						t.Error("ErrNoCache should not detach")
					}
					return "c1", ErrNoCache
				},
				noErr:   true,
				wantVal: "c1",
				sleep:   time.Millisecond * 100,
			},
			{
				name: "ErrNoCache handling",
				key:  "c",
				c:    fn,
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
				c:    fn,
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
				c:    fn,
				fn: func(ctx context.Context) (interface{}, error) {
					panic("booommm")
					return "ok", nil
				},
				wantVal: "",
				wantErr: "booommm",
				sleep:   time.Millisecond,
			},
			{
				name: "should return same custom error",
				key:  "err",
				c:    fn,
				fn: func(ctx context.Context) (interface{}, error) {
					return "abc", errCustomTest
				},
				wantVal:      "",
				wantErrExact: errCustomTest,
				sleep:        time.Millisecond,
			},
		}
		for _, tt := range tests {
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
	})
}

func DoTestFuncDoConcurrent(name string, t *testing.T, c Cache, m, n int, sleep time.Duration) {
	t.Run(name+"FuncDoConcurrent", func(t *testing.T) {
		var (
			fn        = NewFunc(c, time.Second, time.Second, time.Second*5)
			ctx       = context.Background()
			called    = make(chan int, n*m)
			responded = make(chan int, n*m)
		)
		g, ctx := errgroup.WithContext(ctx)
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				(func(j string) {
					g.Go(func() error {
						var val string
						if err := fn.Do(ctx, j, func(_ context.Context) (interface{}, error) {
							time.Sleep(sleep)
							called <- 1
							return "foo" + j, nil
						}, &val); err != nil || val != "foo"+j {
							t.Error(val, err, "wrong value")
							return nil
						}
						responded <- 1
						return nil
					})
				})(strconv.Itoa(j))
			}
		}
		if err := g.Wait(); err != nil {
			t.Error(err)
		}
		if len(called) != m {
			t.Errorf(" = %v, want %v", len(called), m)
		}
		if len(responded) != m*n {
			t.Errorf(" = %v, want %v", len(responded), m*n)
		}
	})
}
