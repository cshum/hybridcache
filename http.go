package cache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

type HTTP struct {
	// Cache adapter
	Cache Cache

	// WaitFor wait timeout for the func call to complete
	WaitFor time.Duration

	// FreshFor best-before duration of cache before the next refresh
	FreshFor time.Duration

	// TTL duration for cache to stay
	TTL time.Duration

	// RequestKey function generates string key from incoming request
	// by default request URL is used as key
	RequestKey func(*http.Request) string

	// AcceptRequest optional function determine request should be handled
	// by default only GET requests are handled
	AcceptRequest func(*http.Request) bool

	// AcceptResponse function determine response should be cached
	// by default only status code < 400 response are cached
	AcceptResponse func(*http.Response) bool

	// ErrorHandler function handles errors
	// by default context deadline exceeded result 408 error, or 400 error for anything else
	ErrorHandler func(http.ResponseWriter, *http.Request, error)
}

func NewHTTP(c Cache, waitFor, freshFor, ttl time.Duration) *HTTP {
	return &HTTP{
		Cache:    c,
		WaitFor:  waitFor,
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

// Handler is the HTTP cache middleware handler
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
				err = ErrNoCache
			}
			return
		}, h.WaitFor, h.FreshFor, h.TTL); err != nil || p == nil {
			if h.ErrorHandler != nil {
				h.ErrorHandler(w, r, err)
			} else if err == context.DeadlineExceeded {
				w.WriteHeader(http.StatusRequestTimeout)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
			return
		}
		for k, v := range p.Header {
			w.Header().Set(k, strings.Join(v, ","))
		}
		w.WriteHeader(p.StatusCode)
		_, _ = w.Write(p.Value)
	})
}
