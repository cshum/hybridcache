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
	Timeout        time.Duration
	MaxSize        int64
	FreshFor       time.Duration
	TTL            time.Duration
	KeyHash        func(r *http.Request) string
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
		if h.KeyHash != nil {
			key = h.KeyHash(r)
		}
		if v, err := getPayload(h.Cache, key); err == nil {
			overrideHeader(w.Header(), v.Header)
			w.WriteHeader(v.StatusCode)
			_, _ = w.Write(v.Value)

			if v.NeedRefresh() {
				if h.Timeout > 0 {
					ctx, cancel = context.WithTimeout(ctx, h.Timeout)
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
					_ = setPayload(h.Cache, key, newPayload(ww.Body.Bytes(), h.FreshFor).
						WithHeader(res.Header, res.StatusCode), h.TTL)
				}()
			}
			return
		}
		if h.Timeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, h.Timeout)
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
			_ = setPayload(
				h.Cache, key, newPayload(val, h.FreshFor).
					WithHeader(res.Header, res.StatusCode), h.TTL)
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

func NewHTTP(c Cache, freshFor, ttl time.Duration) *HTTP {
	return &HTTP{
		Cache:    c,
		FreshFor: freshFor,
		TTL:      ttl,
		KeyHash: func(r *http.Request) string {
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
