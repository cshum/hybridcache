package cache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

type HTTP struct {
	Cache          Cache
	RequestTimeout time.Duration
	MaxSize        int64
	CacheTimeout   time.Duration
	CacheTTL       time.Duration
	Hasher         func(r *http.Request) string
	AcceptRequest  func(r *http.Request) bool
	AcceptResponse func(*http.Response) bool
}

func (h *HTTP) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.AcceptRequest != nil && !h.AcceptRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		var (
			key    = r.RequestURI
			ctx    = DetachContext(r.Context())
			cancel = func() {}
		)
		if h.Hasher != nil {
			key = h.Hasher(r)
		}
		if v, err := GetPayload(h.Cache, key); err == nil {
			overrideHeader(w.Header(), v.Header)
			w.WriteHeader(v.StatusCode)
			_, _ = w.Write(v.Value)

			if v.IsExpired() {
				if h.RequestTimeout > 0 {
					ctx, cancel = context.WithTimeout(ctx, h.RequestTimeout)
				}
				rr := r.WithContext(ctx)
				go func() {
					defer cancel()
					ww := httptest.NewRecorder()
					next.ServeHTTP(ww, rr)
					res := ww.Result()
					if !h.acceptResponse(res) {
						return
					}
					_ = SetPayload(h.Cache, key, NewPayload(ww.Body.Bytes(), h.CacheTimeout).
						WithHeader(res.Header, res.StatusCode), h.CacheTTL)
				}()
			}
			return
		}
		if h.RequestTimeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, h.RequestTimeout)
		}
		defer cancel()
		ww := httptest.NewRecorder()
		next.ServeHTTP(ww, r)
		res := ww.Result()
		val := ww.Body.Bytes()
		overrideHeader(w.Header(), res.Header)
		w.WriteHeader(res.StatusCode)
		_, _ = w.Write(val)

		if !h.acceptResponse(res) {
			return
		}
		go func() {
			_ = SetPayload(
				h.Cache, key, NewPayload(val, h.CacheTimeout).
					WithHeader(res.Header, res.StatusCode), h.CacheTTL)
		}()
	})
}

func overrideHeader(dest, source http.Header) {
	for k, v := range source {
		dest.Set(k, strings.Join(v, ","))
	}
}

func (h *HTTP) acceptResponse(res *http.Response) (ok bool) {
	if h.AcceptResponse != nil && !h.AcceptResponse(res) {
		return
	}
	if h.MaxSize > 0 && res.ContentLength > h.MaxSize {
		return
	}
	ok = true
	return
}

func NewHTTP(c Cache, timeout, ttl time.Duration) *HTTP {
	return &HTTP{
		Cache:        c,
		CacheTimeout: timeout,
		CacheTTL:     ttl,
		Hasher: func(r *http.Request) string {
			return r.RequestURI
		},
		AcceptRequest: func(r *http.Request) bool {
			return r.Method == http.MethodGet
		},
		AcceptResponse: func(res *http.Response) bool {
			return res.StatusCode < 400
		},
	}
}
