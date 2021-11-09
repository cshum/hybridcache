package cache

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func sortQueryString(URL *url.URL) {
	params := URL.Query()
	for _, param := range params {
		sort.Slice(param, func(i, j int) bool {
			return param[i] < param[j]
		})
	}
	URL.RawQuery = params.Encode()
}

func TestHTTP_Handler(t *testing.T) {
	counter := 0
	httpTestHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("value %v", counter)))
	})

	c := NewHTTP(NewMemory(10, int64(10<<20), -1), time.Minute, time.Hour)
	c.RequestKey = func(r *http.Request) string {
		sortQueryString(r.URL)
		return r.URL.String()
	}
	c.AcceptResponse = func(res *http.Response) bool {
		body, _ := ioutil.ReadAll(res.Body)
		defer res.Body.Close()
		reader := ioutil.NopCloser(bytes.NewBuffer(body))
		res.Body = reader
		return strings.Contains(string(body), "value 8")
	}

	handler := c.Handler(httpTestHandler)

	tests := []struct {
		name     string
		url      string
		method   string
		wantBody string
		wantCode int
	}{
		{
			"new response",
			"http://foo.bar/test-1",
			"GET",
			"value 1",
			200,
		},
		{
			"new response",
			"http://foo.bar/test-2",
			"GET",
			"value 2",
			200,
		},
		{
			"returns cached response",
			"http://foo.bar/test-2",
			"GET",
			"value 2",
			200,
		},
		{
			"new response",
			"http://foo.bar/test-3?zaz=baz&baz=zaz",
			"GET",
			"value 4",
			200,
		},
		{
			"returns cached response with custom RequestKey",
			"http://foo.bar/test-3?baz=zaz&zaz=baz",
			"GET",
			"value 4",
			200,
		},
		{
			"new response",
			"http://foo.bar/test-3",
			"POST",
			"value 6",
			200,
		},
		{
			"POST returns new response",
			"http://foo.bar/test-3",
			"POST",
			"value 7",
			200,
		},
		{
			"new response",
			"http://foo.bar/test-3",
			"GET",
			"value 8",
			200,
		},
		{
			"new response based on previous no cache",
			"http://foo.bar/test-3",
			"GET",
			"value 9",
			200,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter++
			var r *http.Request
			var err error

			reader := bytes.NewReader([]byte{})
			r, err = http.NewRequest(tt.method, tt.url, reader)
			if err != nil {
				t.Error(err)
				return
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			time.Sleep(time.Millisecond)

			if !reflect.DeepEqual(w.Code, tt.wantCode) {
				t.Errorf(" = %v, want %v", w.Code, tt.wantCode)
				return
			}
			if !reflect.DeepEqual(w.Body.String(), tt.wantBody) {
				t.Errorf(" = %v, want %v", w.Body.String(), tt.wantBody)
			}
		})
	}
}
