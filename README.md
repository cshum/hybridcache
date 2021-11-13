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

Redis upstream provides cache coordination and call suppression across multiple servers, while Memory downstream enables the fastest response time for frequent cache hit. 
Shall the Redis upstream failed, memory downstream will still operate independently without service disruption.

```go
// wrap function call with hybrid cache
cacheFunc := cache.NewFunc(hybridCache, time.Seconds*20, time.Minute, time.Hour)
// 20 seconds execution timeout, 1 minute fresh-for timeout, 1 hour ttl
var items []*Items
someKey := fmt.Sprintf("key-%d", id)
if err := cacheFunc.Do(ctx, someKey, func(ctx context.Context) (interface{}, error) {
	return someDBOperations(ctx, id)
}, &items); err != nil {
	return err
}
for _, item := range items {
	...
}


// use as an HTTP middleware
cacheHandler := cache.NewHTTP(hybridCache, time.Seconds*30, time.Minute, time.Hour*12).Handler
// 30 seconds request timeout, 1 minute fresh-for timeout, 12 hour ttl
// setting a high ttl will enable "always online" in case of service disruption.
// content will lazy refresh in background (goroutine) after fresh-for timeout

r := mux.NewRouter()
r.Use(cacheHandler)
r.Mount("/", myWebServices)
http.ListenAndServe(":3001", r)
```

A simple function wrapper or HTTP middleware gives you under the hood:

* Lazy background refresh with timeout - after fresh-for timeout exceeded, the next cache hit will trigger a refresh in goroutine, where context deadline is detached from parent and based on wait-for timeout. 
* Cache stampede prevention - uses singleflight for memory call suppression and `SEX NX` for redis.
* Marshal and unmarshal options for function calls - default to msgpack, with options to configure your own.