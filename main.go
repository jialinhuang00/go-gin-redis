package main

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var cache = make(map[string]string)    // redis-like
var hitCountMap = make(map[string]int) // frequency check
var mu sync.RWMutex                    // locker

func simulateHeavyComputation() int {
	// just sleep LOL
	time.Sleep(5 * time.Second)
	return 42
}

func heavyMessage(c *gin.Context) {
	// Start the timer to calculate the duration
	startTime := time.Now()

	cacheKey := "heavy-message"
	hitCountKey := cacheKey + "-hits"

	// read lock for reading
	mu.RLock()
	hitCount, exists := hitCountMap[hitCountKey]
	mu.RUnlock()

	// If no hits found, initialize it
	if !exists {
		hitCount = 0
	}

	var message string
	var source string

	// Logic to handle cache and computation
	if hitCount >= 1 {
		// read lock for reading
		mu.RLock()
		cachedMessage, cached := cache[cacheKey]
		mu.RUnlock()

		if !cached {
			// If no cache, return the result and store in cache
			result := simulateHeavyComputation()
			// read-write lock for writing
			mu.Lock()
			cache[cacheKey] = strconv.Itoa(result)
			mu.Unlock()
			message = strconv.Itoa(result)
			source = "computed and stored in cache"
		} else {
			// If matched, return from cache
			message = cachedMessage
			source = "cache"
		}
	} else {
		// It's your first time accessing
		// read-write lock for writing
		mu.Lock()
		hitCountMap[hitCountKey] = hitCount + 1
		mu.Unlock()

		// Might be a one-time access, do not store in cache!
		result := simulateHeavyComputation()
		message = strconv.Itoa(result)
		source = "computed without cache (first access)"
	}

	// Calculate the duration from request to response
	duration := time.Since(startTime)

	// Return the response with the duration
	c.JSON(http.StatusOK, gin.H{
		"message":  message,
		"source":   source,
		"duration": duration.Seconds(), // Duration in seconds
	})
}
func prune(c *gin.Context) {

	mu.Lock()
	defer mu.Unlock()

	clear(cache)
	clear(hitCountMap)

	c.JSON(http.StatusOK, gin.H{
		"message": "Cache and hit count cleaned up successfully",
	})
}

func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Yes, I'm woking",
	})
}

func main() {
	// gin framework
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * 60 * 60,
	}))

	r.GET("/heavy", heavyMessage)
	r.GET("/prune", prune)
	r.GET("/", health)

	r.Run(":80")
}
