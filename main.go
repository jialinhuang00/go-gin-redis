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

type CacheEntry struct {
	key             string
	value           string
	consecutiveHits int
	isBeingUsed     bool
}

type Cache struct {
	mu           sync.RWMutex
	items        map[string]*list.Element
	evictionList *list.List // LRU
	accessOrder  map[string]time.Time
}

func NewCache() *Cache {
	return &Cache{
		items:        make(map[string]*list.Element),
		evictionList: list.New(),
		accessOrder:  make(map[string]time.Time),
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
		c.accessOrder[key] = time.Now()

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
		c.accessOrder[key] = time.Now()
		return
	}

	// no room to store, delete one
	if c.evictionList.Len() >= CACHE_SIZE {
		c.evictOne()
	}

	entry := &CacheEntry{
		key:             key,
		value:           value,
		consecutiveHits: 1,
		isBeingUsed:     false,
	}

	element := c.evictionList.PushFront(entry)
	c.items[key] = element
	c.accessOrder[key] = time.Now()
}

func (c *Cache) evictOne() {

	// delete existing but not being used.
	for e := c.evictionList.Back(); e != nil; e = e.Prev() {
		entry := e.Value.(*CacheEntry)
		if !entry.isBeingUsed {
			c.evictionList.Remove(e)
			delete(c.items, entry.key)
			delete(c.accessOrder, entry.key)
			return
		}
	}
	// if all are being used, just check the last one.
	if element := c.evictionList.Back(); element != nil {
		entry := element.Value.(*CacheEntry)
		c.evictionList.Remove(element)
		delete(c.items, entry.key)
		delete(c.accessOrder, entry.key)
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

func (c *Cache) GetStatus() []map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := make([]map[string]any, 0, c.evictionList.Len())

	for e := c.evictionList.Front(); e != nil; e = e.Next() {
		entry := e.Value.(*CacheEntry)
		item := map[string]any{
			"key":             entry.key,
			"consecutiveHits": entry.consecutiveHits,
			"isBeingUsed":     entry.isBeingUsed,
			"lastAccessed":    c.accessOrder[entry.key],
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
	c.accessOrder = make(map[string]time.Time)
}

var cacheManager = NewCache()
var lastAccessedKeys = make(map[string]time.Time)
var lastAccessMu sync.RWMutex

func simulateHeavyComputation(key string) string {
	time.Sleep(3 * time.Second)
	return fmt.Sprintf("heavy message %s", key)
}

func heavyMessage(c *gin.Context) {
	startTime := time.Now()

	messageKey := c.DefaultQuery("key", "default")
	cacheKey := fmt.Sprintf("heavy-message-%s", messageKey)

	lastAccessMu.RLock()
	lastAccess, exists := lastAccessedKeys[cacheKey]
	lastAccessMu.RUnlock()

	// outdated or doesn't exist?
	if !exists || time.Since(lastAccess) > 30*time.Second {
		cacheManager.ResetConsecutiveHits(cacheKey)
	}

	lastAccessMu.Lock()
	lastAccessedKeys[cacheKey] = time.Now()
	lastAccessMu.Unlock()

	// check cache first
	cachedMessage, hit := cacheManager.Get(cacheKey)

	var message string
	var source string

	if hit {
		message = cachedMessage
		source = "cache"
	} else {
		message = simulateHeavyComputation(messageKey)

		cacheManager.Set(cacheKey, message)
		source = "computed and stored in cache"
	}

	duration := time.Since(startTime)

	isBeingUsed := false
	consecutiveHits := 0
	cacheManager.mu.RLock()
	if element, exists := cacheManager.items[cacheKey]; exists {
		entry := element.Value.(*CacheEntry)
		isBeingUsed = entry.isBeingUsed
		consecutiveHits = entry.consecutiveHits
	}
	cacheManager.mu.RUnlock()

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

func status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"cacheStatus": cacheManager.GetStatus(),
		"cacheSize":   cacheManager.evictionList.Len(),
		"maxSize":     CACHE_SIZE,
	})
}

func prune(c *gin.Context) {
	cacheManager.Clear()

	lastAccessMu.Lock()
	lastAccessedKeys = make(map[string]time.Time)
	lastAccessMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"message": "Cache cleaned up successfully",
	})
}

func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Yes, I'm working",
	})
}

func main() {
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
	r.GET("/status", status)
	r.GET("/prune", prune)
	r.GET("/", health)

	r.Run(":80")
}
