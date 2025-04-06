package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"go-gin-cache/pkg/cache"
	"go-gin-cache/pkg/source"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// N, it doesn't expose to the API interface.
const CACHE_SIZE = 5

// It can be changed by the API.
var expirationTime = 30 * time.Second
var expirationTimeMu sync.RWMutex

var cacheManager = cache.New(CACHE_SIZE)
var anotherCacheManager = cache.New(CACHE_SIZE)

func simulateHeavyComputation(key string) string {
	time.Sleep(3 * time.Second)
	return fmt.Sprintf("heavy message %s", key)
}

// getCacheManager returns the appropriate cache manager based on the endpoint
func getCacheManager(endpoint string) *cache.Cache {
	if strings.Contains(endpoint, "another") {
		return anotherCacheManager
	}
	return cacheManager
}

// handler
func heavyMessage(c *gin.Context) {
	startTime := time.Now()

	messageKey := c.DefaultQuery("key", "default")
	cacheKey := fmt.Sprintf("%s-message-%s", c.Request.URL.Path, messageKey)

	// Get current expiration time
	expirationTimeMu.RLock()
	currentExpirationTime := expirationTime
	expirationTimeMu.RUnlock()

	var message string
	var cacheSource source.CacheSource
	var isBeingUsed bool = false
	var consecutiveHits int = 0

	manager := getCacheManager(c.Request.URL.Path)

	// Check if key exists in cache
	if value, exists := manager.Get(cacheKey); exists {
		lastAccess, _ := manager.GetLastAccessTime(cacheKey)

		// it's outdated?
		if time.Since(lastAccess) > currentExpirationTime {
			manager.ResetConsecutiveHits(cacheKey)
			consecutiveHits = 1
		} else {
			manager.IncrementConsecutiveHits(cacheKey)
			consecutiveHits, _ = manager.GetConsecutiveHits(cacheKey)
		}

		message = value
		cacheSource = source.Hit
		isBeingUsed = consecutiveHits >= 2
		manager.Update(cacheKey, value)
	} else {
		message = simulateHeavyComputation(messageKey)
		cacheSource = source.Computed
		manager.Add(cacheKey, message)
		consecutiveHits = 1
	}

	duration := time.Since(startTime)

	c.JSON(http.StatusOK, gin.H{
		"key":             messageKey,
		"message":         message,
		"source":          cacheSource,
		"duration":        duration.Seconds(),
		"isBeingUsed":     isBeingUsed,
		"consecutiveHits": consecutiveHits,
		"cacheSize":       manager.Len(),
		"maxSize":         CACHE_SIZE,
	})
}

// New endpoint to set expiration time
func setExpirationTime(c *gin.Context) {
	var request struct {
		Time int `json:"time" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid input. Please provide a valid 'time' value in seconds.",
		})
		return
	}

	// Validate time is reasonable (e.g., not negative or too large)
	if request.Time <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Time must be a positive value in seconds.",
		})
		return
	}

	// Set the new expiration time
	expirationTimeMu.Lock()
	expirationTime = time.Duration(request.Time) * time.Second
	expirationTimeMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"message":        "Expiration time updated successfully",
		"expirationTime": request.Time,
		"unit":           "seconds",
	})
}

// get setup and current cache status
func status(c *gin.Context) {
	// Get current expiration time
	expirationTimeMu.RLock()
	currentExpirationTime := expirationTime
	expirationTimeMu.RUnlock()

	manager := getCacheManager(c.Request.URL.Path)

	c.JSON(http.StatusOK, gin.H{
		"cacheStatus":    manager.GetStatus(),
		"cacheSize":      manager.Len(),
		"maxSize":        CACHE_SIZE,
		"expirationTime": int(currentExpirationTime.Seconds()),
	})
}

// clean all
func prune(c *gin.Context) {
	manager := getCacheManager(c.Request.URL.Path)
	manager.Clear()

	c.JSON(http.StatusOK, gin.H{
		"message": "Cache cleaned up successfully",
	})
}

// just a health check
func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Yes, I'm working",
	})
}

func main() {
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * 60 * 60,
	}))

	// First set of endpoints
	r.GET("/heavy", heavyMessage)
	r.GET("/heavy/status", status)
	r.GET("/heavy/prune", prune)
	r.POST("/heavy/expiration", setExpirationTime)

	// Second set of endpoints
	r.GET("/another-heavy", heavyMessage)
	r.GET("/another-heavy/status", status)
	r.GET("/another-heavy/prune", prune)
	r.POST("/another-heavy/expiration", setExpirationTime)

	r.GET("/", health)

	r.Run(":80")
}
