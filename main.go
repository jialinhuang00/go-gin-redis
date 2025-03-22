package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

var rdb *redis.Client
var ctx = context.Background()

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

	// Remember how many times the GET `/heavy` endpoint was hit
	hitCount, err := rdb.Get(ctx, hitCountKey).Result()

	// If nil, set the counter to 0
	if err == redis.Nil {
		hitCount = "0"
	} else if err != nil {
		log.Println("Redis get error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis error"})
		return
	}

	// Convert str to integer
	hitCountInt, _ := strconv.Atoi(hitCount)

	var message string
	var source string

	// Logic to handle cache and computation
	if hitCountInt >= 1 {
		cachedMessage, err := rdb.Get(ctx, cacheKey).Result()
		if err == redis.Nil {
			// If no cache, return the result and store in Redis
			result := simulateHeavyComputation()
			err := rdb.Set(ctx, cacheKey, strconv.Itoa(result), 0).Err()
			if err != nil {
				log.Println("Redis set error:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis set error"})
				return
			}
			message = strconv.Itoa(result)
			source = "computed and stored in cache"
		} else if err != nil {
			log.Println("Redis get error:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis error"})
		} else {
			// If matched, return from Redis cache
			message = cachedMessage
			source = "cache"
		}
	} else {
		// It's your first time accessing
		err := rdb.Set(ctx, hitCountKey, hitCountInt+1, 0).Err()
		if err != nil {
			log.Println("Redis set error:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis error"})
			return
		}

		// Might be a one-time access, don’t store in Redis yet
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

func main() {
	// init redis
	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})
	// gin framework
	r := gin.Default()

	r.GET("/heavy", heavyMessage)

	r.Run(":80")
}
