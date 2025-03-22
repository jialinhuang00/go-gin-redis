# Prepare Redis (for MacOS)

0. brew install redis
1. brew services start redis
2. redis-cli flushall (cleanup)

# Download and run
0. clone this
1. go mod tidy (if needed)
2. go run main go
3. program itself is already connected to redis (no worries)

# Open new Terminal

0. curl "http://localhost:8080/heavy" (repeatedly)
    1. first time access: 5 sec
    2. second time access: 5 sec
    3. else: 0.0003 sec
1. you might want to try again 
    1. redis-cli flushall
