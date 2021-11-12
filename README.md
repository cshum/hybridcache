# HybridCache

A multi-level cache library with cache stampede prevention for Go

```go
import "github.com/cshum/hybridcache"

// Redis cache based on Redigo
redisCache := NewRedis(&redis.Pool{...})

// Memory cache based on Ristretto
memoryCache := cache.NewMemory(1e5, 1<<29, time.Hour)
// bounded maximum ~100,000 items, ~500mb memory size, 1 hour ttl

// create a hybrid cache that takes redis upstream + in-memory downstream
hybridCache := cache.NewHybrid(redisCache, memoryCache)
```

Redis upstream provides cache coordination and call suppression across multiple servers, while Memory downstream enables the fastest response time for frequent cache hit. 
Shall the Redis upstream failed, memory downstream will still operate independently without service disruption.

```go
// wrap arbitrary function calls with hybrid cache
cacheFunc := cache.NewFunc(hybridCache, time.Seconds*20, time.Minute, time.Hour)
// 20 seconds execution timeout, 1 minute best-before timeout, 1 hour ttl
var items []*Items
if err := cacheFunc.Do(ctx, "some-key", func(ctx context.Context) (interface{}, error) {
	return someHeavyOperations(ctx)
}, &items); err != nil {
	return err
}
for _, item := range items {
	...
}
...

// use as an HTTP middleware
cacheHandler := cache.NewHTTP(hybridCache, time.Seconds*30, time.Minute, time.Hour*12).Handler
// 30 seconds request timeout, 1 minute best-before timeout, 12 hour ttl
// setting a high ttl will enable "always online" in case of service disruption.
// content will lazy refresh in background (goroutine) after best-before timeout

r := chi.NewRouter()
r.Use(cacheHandler)
r.Mount("/", myWebServices)
http.ListenAndServe(":3001", r)
```

A simple function wrapper or HTTP middleware gives you a lot under the hood:

* adaptive caching between in-memory and redis
* lazy background refresh with liveness
* cache stampede prevention / call suppression
* marshalling

## License

MIT