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
	cacheKey := "heavy-message"
	hitCountKey := cacheKey + "-hits"

	// remember how many times that GET `/heavy` endpoint
	hitCount, err := rdb.Get(ctx, hitCountKey).Result()

	// if nil, set counter to 0
	if err == redis.Nil {
		hitCount = "0"
	} else if err != nil {
		log.Println("Redis get error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis error"})
		return
	}

	// convert str to integer
	hitCountInt, _ := strconv.Atoi(hitCount)

	if hitCountInt >= 1 {
		cachedMessage, err := rdb.Get(ctx, cacheKey).Result()
		if err == redis.Nil {
			// if no cache, return the result and store in redis
			result := simulateHeavyComputation()
			err := rdb.Set(ctx, cacheKey, strconv.Itoa(result), 0).Err()
			if err != nil {
				log.Println("Redis set error:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis set error"})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"message": result,
				"source":  "computed and stored in cache",
			})
		} else if err != nil {
			log.Println("Redis get error:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis error"})
		} else {
			// if matched, return from redis cache
			c.JSON(http.StatusOK, gin.H{
				"message": cachedMessage,
				"source":  "cache",
			})
		}
	} else {
		// it's your first time to get here
		err := rdb.Set(ctx, hitCountKey, hitCountInt+1, 0).Err()
		if err != nil {
			log.Println("Redis set error:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis error"})
			return
		}

		// might be a one-time access
		// don’t need to store it in Redis yet,
		result := simulateHeavyComputation()
		c.JSON(http.StatusOK, gin.H{
			"message": result,
			"source":  "computed without cache (first access)",
		})
	}
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

	r.Run(":8080")
}
