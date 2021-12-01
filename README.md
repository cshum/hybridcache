# HybridCache

A multi-level cache library with cache stampede prevention for Go

```go
import "github.com/cshum/hybridcache"

// Redis cache adapter based on Redigo
redisCache := cache.NewRedis(&redis.Pool{...})

// Memory cache adapter based on Ristretto
memoryCache := cache.NewMemory(1e5, 1<<29, time.Hour)
// bounded maximum ~100,000 items, ~500mb memory size, 1 hour ttl

// Hybrid cache adapter with Redis upstream + Memory downstream
hybridCache := cache.NewHybrid(redisCache, memoryCache)
```

The hybrid combination allows Redis upstream coordinate across multiple servers, while Memory downstream ensures minimal network I/O which brings the fastest response time. 
Shall the Redis upstream failed, memory downstream will still operate independently without service disruption.
```go
// cache function client
cacheFunc := cache.NewFunc(hybridCache, time.Seconds*20, time.Minute, time.Hour)
// 20 seconds execution timeout, 1 minute fresh-for timeout, 1 hour ttl
var items []*Items
someKey := fmt.Sprintf("key-%d", id)

// wrap function call with hybrid cache
if err := cacheFunc.Do(ctx, someKey, func(ctx context.Context) (interface{}, error) {
	return someHeavyOperations(ctx, id)
}, &items); err != nil {
	return err
}
for _, item := range items {
	...
}

// cache http client
cacheHTTP := cache.NewHTTP(hybridCache, time.Seconds*30, time.Minute, time.Hour*12)
// 30 seconds request timeout, 1 minute fresh-for timeout, 12 hour ttl
// setting a high ttl will enable "always online" in case of service disruption.
// content will lazy refresh in background (goroutine) after fresh-for timeout

// use as an HTTP middleware
r := mux.NewRouter()
r.Use(cacheHTTP.Handler)
r.Mount("/", myWebServices)
http.ListenAndServe(":3001", r)

// wraps a http.RoundTripper
cachedTransport := cacheHTTP.RoundTripper(http.DefaultTransport)
```
A simple function wrapper or HTTP middleware gives you under the hood:

* Lazy background refresh with timeout - after fresh-for timeout exceeded, the next cache hit will trigger a refresh in goroutine, where context deadline is detached from parent and based on wait-for timeout. 
* Cache stampede prevention - uses singleflight for memory call suppression and `SEX NX` for redis.
* Marshal and unmarshal options for function calls - default to msgpack, with options to configure your own.


Conditional caching with `cache.ErrNoCache`:

```go
var items []*Items
someKey := fmt.Sprintf("key-%d", id)
if err := cacheFunc.Do(ctx, someKey, func(ctx context.Context) (interface{}, error) {
	start := time.Now()
	items, err := someHeavyOperations(ctx, id)
	if err != nil {
		return nil, err
	}
	if time.Since(start) < time.Millisecond*300 {
		// no cache if response within 300 milliseconds
		return items, cache.ErrNoCache
	}
	return items, nil
}, &items); err != nil {
	// ErrNoCache will NOT appear outside
	return err
}
```
More options:
```go
cacheFunc := cache.NewFunc(hybridCache, time.Seconds*20, time.Minute, time.Hour)
cacheFunc.Marshal = json.Marshal
cacheFunc.Unmarshal = json.Unmarshal
// custom Marshal Unmarshal function, default msgpack


h := cache.NewHTTP(hybridCache, time.Seconds*30, time.Minute, time.Hour*12)
h.RequestKey = func(r *http.Request) string {
	// cache key by url excluding query params
	return strings.Split(r.URL.String(), "?")[0]
}
h.AcceptRequest = func(r *http.Request) bool {
	if strings.Contains(r.URL.RawQuery, "nocache") {
		// no cache if nocache appears in query
		return false
	}
	return true
}
h.AcceptResponse = func(res *http.Response) bool {
	if res.StatusCode != http.StatusOK {
		// no cache if status code not 200
		return false
	}
	if res.ContentLength >= 1<<20 {
		// no cache if over 1 MB
		return false
	}
	return true
}
cacheHandler := h.Handler
```