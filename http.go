package cache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

type HTTP struct {
	Cache    Cache
	Timeout  time.Duration
	FreshFor time.Duration
	TTL      time.Duration

	GetKey          func(*http.Request) string
	IsHandleRequest func(*http.Request) bool
	IsNoCache       func(*http.Response) bool
}

func (h *HTTP) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			key = r.RequestURI
			ctx = r.Context()
			p   *payload
			err error
		)
		if h.IsHandleRequest != nil && !h.IsHandleRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		if h.GetKey != nil {
			key = h.GetKey(r)
		}
		if p, err = do(ctx, h.Cache, key, func(ctx context.Context) (p *payload, err error) {
			var (
				ww  = httptest.NewRecorder()
				rr  = r.WithContext(ctx)
				res *http.Response
			)
			next.ServeHTTP(ww, rr)
			res = ww.Result()
			p = newPayload(ww.Body.Bytes()).WithHeader(res.Header, res.StatusCode)
			if h.IsNoCache != nil && h.IsNoCache(res) {
				err = NoCache
			}
			return
		}, h.Timeout, h.FreshFor, h.TTL); err != nil || p == nil {
			next.ServeHTTP(w, r)
			return
		}
		overrideHeader(w.Header(), p.Header)
		w.WriteHeader(p.StatusCode)
		_, _ = w.Write(p.Value)
	})
}

func overrideHeader(dest, source http.Header) {
	for k, v := range source {
		dest.Set(k, strings.Join(v, ","))
	}
}

func NewHTTP(c Cache, freshFor, ttl time.Duration) *HTTP {
	return &HTTP{
		Cache:    c,
		FreshFor: freshFor,
		TTL:      ttl,
		GetKey: func(r *http.Request) string {
			return r.RequestURI
		},
		IsHandleRequest: func(r *http.Request) bool {
			return r.Method == http.MethodGet
		},
		IsNoCache: func(res *http.Response) bool {
			return res.StatusCode < 400
		},
	}
}
