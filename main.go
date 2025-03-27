package main

import (
	"container/list"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// N
const CACHE_SIZE = 5

var expirationTime = 30 * time.Second
var expirationTimeMu sync.RWMutex

type CacheEntry struct {
	key             string
	value           string
	consecutiveHits int
	isBeingUsed     bool
}

type Cache struct {
	mu             sync.RWMutex
	items          map[string]*list.Element
	evictionList   *list.List // LRU
	lastAccessTime map[string]time.Time
}

func NewCache() *Cache {
	return &Cache{
		items:          make(map[string]*list.Element),
		evictionList:   list.New(),
		lastAccessTime: make(map[string]time.Time),
	}
}

func (c *Cache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if element, exists := c.items[key]; exists {
		entry := element.Value.(*CacheEntry)
		entry.consecutiveHits++

		if entry.consecutiveHits >= 2 {
			entry.isBeingUsed = true
		}

		c.evictionList.MoveToFront(element)
		c.lastAccessTime[key] = time.Now()

		return entry.value, true
	}
	return "", false
}

func (c *Cache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, exists := c.items[key]; exists {
		c.evictionList.MoveToFront(element)
		entry := element.Value.(*CacheEntry)
		entry.value = value
		c.lastAccessTime[key] = time.Now()
		return
	}

	// no room to store, delete one
	if c.evictionList.Len() >= CACHE_SIZE {
		c.EvictOne()
	}

	entry := &CacheEntry{
		key:             key,
		value:           value,
		consecutiveHits: 1,
		isBeingUsed:     false,
	}

	element := c.evictionList.PushFront(entry)
	c.items[key] = element
	c.lastAccessTime[key] = time.Now()
}

func (c *Cache) EvictOne() {
	// delete existing but not being used.
	for e := c.evictionList.Back(); e != nil; e = e.Prev() {
		entry := e.Value.(*CacheEntry)
		if !entry.isBeingUsed {
			c.evictionList.Remove(e)
			delete(c.items, entry.key)
			delete(c.lastAccessTime, entry.key)
			return
		}
	}
	// if all are being used, just check the last one.
	if element := c.evictionList.Back(); element != nil {
		entry := element.Value.(*CacheEntry)
		c.evictionList.Remove(element)
		delete(c.items, entry.key)
		delete(c.lastAccessTime, entry.key)
	}
}

func (c *Cache) ResetConsecutiveHits(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, exists := c.items[key]; exists {
		entry := element.Value.(*CacheEntry)
		entry.consecutiveHits = 1
		entry.isBeingUsed = false
	}
}

func (c *Cache) GetStatus() []map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := make([]map[string]any, 0, c.evictionList.Len())

	for e := c.evictionList.Front(); e != nil; e = e.Next() {
		entry := e.Value.(*CacheEntry)
		item := map[string]any{
			"key":             entry.key,
			"consecutiveHits": entry.consecutiveHits,
			"isBeingUsed":     entry.isBeingUsed,
			"lastAccessed":    c.lastAccessTime[entry.key],
		}
		status = append(status, item)
	}

	return status
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.evictionList = list.New()
	c.lastAccessTime = make(map[string]time.Time)
}

var cacheManager = NewCache()

func simulateHeavyComputation(key string) string {
	time.Sleep(3 * time.Second)
	return fmt.Sprintf("heavy message %s", key)
}

// handler
func heavyMessage(c *gin.Context) {
	startTime := time.Now()

	messageKey := c.DefaultQuery("key", "default")
	cacheKey := fmt.Sprintf("heavy-message-%s", messageKey)

	// Get current expiration time
	expirationTimeMu.RLock()
	currentExpirationTime := expirationTime
	expirationTimeMu.RUnlock()

	var message string
	var source string
	var isBeingUsed bool = false
	var consecutiveHits int = 0

	cacheManager.mu.Lock()
	element, hit := cacheManager.items[cacheKey]
	if hit {
		entry := element.Value.(*CacheEntry)
		lastAccess := cacheManager.lastAccessTime[cacheKey]

		// it's outdated?
		if time.Since(lastAccess) > currentExpirationTime {
			entry.consecutiveHits = 1
			entry.isBeingUsed = false
			// its' fresh
		} else {
			entry.consecutiveHits++
		}

		message = entry.value
		source = "cache"
		consecutiveHits = entry.consecutiveHits

		if entry.consecutiveHits >= 2 {
			entry.isBeingUsed = true
		}
		isBeingUsed = entry.isBeingUsed

		cacheManager.evictionList.MoveToFront(element)
		cacheManager.lastAccessTime[cacheKey] = time.Now()
	}
	cacheManager.mu.Unlock()

	if !hit {
		message = simulateHeavyComputation(messageKey)
		source = "computed and stored in cache"
		cacheManager.Set(cacheKey, message)
		consecutiveHits = 1
	}

	duration := time.Since(startTime)

	c.JSON(http.StatusOK, gin.H{
		"key":             messageKey,
		"message":         message,
		"source":          source,
		"duration":        duration.Seconds(),
		"isBeingUsed":     isBeingUsed,
		"consecutiveHits": consecutiveHits,
		"cacheSize":       cacheManager.evictionList.Len(),
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

	c.JSON(http.StatusOK, gin.H{
		"cacheStatus":    cacheManager.GetStatus(),
		"cacheSize":      cacheManager.evictionList.Len(),
		"maxSize":        CACHE_SIZE,
		"expirationTime": int(currentExpirationTime.Seconds()),
	})
}

// clean all
func prune(c *gin.Context) {
	cacheManager.Clear()

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

	r.GET("/heavy", heavyMessage)
	r.GET("/status", status)
	r.GET("/prune", prune)
	r.GET("/", health)
	r.POST("/expirationTime", setExpirationTime)

	r.Run(":80")
}
