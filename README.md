# Download and run
0. clone
1. go mod tidy (if needed)
2. go run main go

# Curl
local run: `curl localhost`

0. health-check 
    ```
    curl https://gin.jialin00.com/
    ```
1. try getting time-consuming resource
    ```
    curl https://gin.jialin00.com/heavy?key=whateveryyouwant
    ```
2. cleanup
    ```
    curl https://gin.jialin00.com/prune
    ```

2. get cache status
    ```
    curl https://gin.jialin00.com/status
    ```
3. set expiration time
    ```
    curl -X POST \
    http://gin.jialin00.com/expirationTime \
    -H 'Content-Type: application/json' \
    -d '{"time": 8}'
    ```

# Or Website
https://jialin00.com/gin-redis
 
