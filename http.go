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
	FreshFor time.Duration
	TTL      time.Duration

	RequestKey     func(*http.Request) string
	AcceptRequest  func(*http.Request) bool
	AcceptResponse func(*http.Response) bool
}

func (h HTTP) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			key = r.URL.String()
			ctx = r.Context()
			p   *payload
			err error
		)
		if h.AcceptRequest != nil && !h.AcceptRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		if h.RequestKey != nil {
			key = h.RequestKey(r)
		}
		if p, err = do(ctx, h.Cache, key, func(ctx context.Context) (p *payload, err error) {
			var (
				ww  = httptest.NewRecorder()
				rr  = r.WithContext(ctx)
				res *http.Response
			)
			next.ServeHTTP(ww, rr)
			res = ww.Result()
			p = newPayload(ww.Body.Bytes())
			p.Header = res.Header
			p.StatusCode = res.StatusCode
			if h.AcceptResponse != nil && !h.AcceptResponse(res) {
				err = NoCache
			}
			return
		}, h.FreshFor, h.TTL); err != nil || p == nil {
			next.ServeHTTP(w, r)
			return
		}
		for k, v := range p.Header {
			w.Header().Set(k, strings.Join(v, ","))
		}
		w.WriteHeader(p.StatusCode)
		_, _ = w.Write(p.Value)
	})
}

func NewHTTP(c Cache, freshFor, ttl time.Duration) *HTTP {
	return &HTTP{
		Cache:    c,
		FreshFor: freshFor,
		TTL:      ttl,
		AcceptRequest: func(r *http.Request) bool {
			return r.Method == http.MethodGet
		},
		AcceptResponse: func(res *http.Response) bool {
			return res.StatusCode < 400
		},
	}
}
