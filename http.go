package cache

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

// HTTP cache client as an HTTP middleware
type HTTP struct {
	// Cache adapter
	Cache Cache

	// WaitFor request timeout
	WaitFor time.Duration

	// FreshFor best-before duration of cache before the next refresh
	FreshFor time.Duration

	// TTL duration for cache to stay
	TTL time.Duration

	// RequestKey function generates string key from incoming request
	//
	// by default request URL is used as key
	RequestKey func(*http.Request) string

	// AcceptRequest optional function determine request should be handled
	//
	// by default only GET requests are handled
	AcceptRequest func(*http.Request) bool

	// AcceptResponse function determine response should be cached
	//
	// by default only status code < 400 response are cached
	AcceptResponse func(*http.Response) bool

	// ErrorHandler function handles errors
	//
	// by default context deadline will result 408 error, 400 error for anything else
	ErrorHandler func(http.ResponseWriter, *http.Request, error)

	// Transport the http.RoundTripper to wrap. Defaults to http.DefaultTransport
	Transport http.RoundTripper
}

// NewHTTP creates cache HTTP middleware client with options:
//	waitFor request timeout,
//	freshFor fresh duration until next refresh,
//	ttl cache time-to-live
func NewHTTP(c Cache, waitFor, freshFor, ttl time.Duration) *HTTP {
	return &HTTP{
		Cache:    c,
		WaitFor:  waitFor,
		FreshFor: freshFor,
		TTL:      ttl,
		RequestKey: func(r *http.Request) string {
			// default using url as key
			return r.URL.String()
		},
		AcceptRequest: func(r *http.Request) bool {
			// default only GET requests will be handled
			return r.Method == http.MethodGet
		},
		AcceptResponse: func(res *http.Response) bool {
			// default status code < 400 will be cached
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

// RoundTripper wraps and returns a http.RoundTripper for cache
func (h HTTP) RoundTripper(transport http.RoundTripper) http.RoundTripper {
	h.Transport = transport
	return h
}

// RoundTrip implements http.RoundTripper
func (h HTTP) RoundTrip(r *http.Request) (*http.Response, error) {
	if h.Transport == nil {
		h.Transport = http.DefaultTransport
	}
	var (
		key = r.URL.String()
		ctx = r.Context()
	)
	if h.AcceptRequest != nil && !h.AcceptRequest(r) {
		return h.Transport.RoundTrip(r)
	}
	if h.RequestKey != nil {
		key = h.RequestKey(r)
	}
	p, err := do(ctx, h.Cache, key, func(ctx context.Context) (p *payload, err error) {
		var (
			rr   = r.WithContext(ctx)
			res  *http.Response
			body []byte
		)
		if res, err = h.Transport.RoundTrip(rr); err != nil {
			return
		}
		if body, err = io.ReadAll(res.Body); err != nil {
			return
		}
		res.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		p = newPayload(body)
		p.Header = res.Header
		p.StatusCode = res.StatusCode
		if h.AcceptResponse != nil && !h.AcceptResponse(res) {
			err = ErrNoCache
		}
		return
	}, h.WaitFor, h.FreshFor, h.TTL)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrNotFound
	}
	header := make(http.Header, 0)
	for k, v := range p.Header {
		header.Set(k, strings.Join(v, ","))
	}
	return &http.Response{
		Status:        http.StatusText(p.StatusCode),
		StatusCode:    p.StatusCode,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          ioutil.NopCloser(bytes.NewBuffer(p.Value)),
		ContentLength: int64(len(p.Value)),
		Request:       r,
		Header:        header,
	}, nil
}
