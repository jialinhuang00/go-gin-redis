# Go Gin Cache

A simple caching system implemented in Go using Gin framework. It demonstrates a basic caching mechanism with features like cache expiration, LRU eviction, and consecutive hits tracking.

## Feat

- Cache size limit (5 items). it's a constant
- LRU (Least Recently Used) eviction policy
- Cache expiration time (default 30 seconds) with POST /expirationTime
- Consecutive hits tracking
- Cache status monitoring
- Cache cleanup functionality


## Download and run
0. clone
1. go mod tidy (if needed)
2. go run main go

## Curl

1. Heavy Message Endpoint
```bash
curl "http://localhost/heavy?key=test1"
```

2. Cache Status Endpoint
```bash
curl "http://localhost/status"
```

3. Cache Cleanup Endpoint
```bash
curl "http://localhost/prune"
```

4. Health Check Endpoint
```bash
curl "http://localhost/"
```

5. Set Expiration Time Endpoint
```bash
curl -X POST "http://localhost/expirationTime" \
     -H "Content-Type: application/json" \
     -d '{"time": 60}'
```

## Or Website (pair with this local server)
https://jialin00.com/gin-redis
 
