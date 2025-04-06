package cache

import (
	"container/list"
	"sync"
	"time"
)

// CacheEntry represents a single cache entry
type CacheEntry struct {
	Key             string
	Value           string
	ConsecutiveHits int
	IsBeingUsed     bool
}

// Cache represents the cache structure
type Cache struct {
	mu             sync.RWMutex
	items          map[string]*list.Element
	evictionList   *list.List // LRU
	lastAccessTime map[string]time.Time
	size           int
}

// New creates a new cache instance with the specified size
func New(size int) *Cache {
	return &Cache{
		items:          make(map[string]*list.Element),
		evictionList:   list.New(),
		lastAccessTime: make(map[string]time.Time),
		size:           size,
	}
}

// Add adds a new item to the cache
func (c *Cache) Add(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// no room to store, delete one
	if c.evictionList.Len() >= c.size {
		c.EvictOne()
	}

	entry := &CacheEntry{
		Key:             key,
		Value:           value,
		ConsecutiveHits: 1,
		IsBeingUsed:     false,
	}

	element := c.evictionList.PushFront(entry)
	c.items[key] = element
	c.lastAccessTime[key] = time.Now()
}

// EvictOne removes one item from the cache using LRU policy
func (c *Cache) EvictOne() {
	// delete existing but not being used
	for e := c.evictionList.Back(); e != nil; e = e.Prev() {
		entry := e.Value.(*CacheEntry)
		if !entry.IsBeingUsed {
			c.evictionList.Remove(e)
			delete(c.items, entry.Key)
			delete(c.lastAccessTime, entry.Key)
			return
		}
	}
	// if all are being used, just check the last one
	if element := c.evictionList.Back(); element != nil {
		entry := element.Value.(*CacheEntry)
		c.evictionList.Remove(element)
		delete(c.items, entry.Key)
		delete(c.lastAccessTime, entry.Key)
	}
}

// GetStatus returns the current status of the cache
func (c *Cache) GetStatus() []map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := make([]map[string]any, 0, c.evictionList.Len())

	for e := c.evictionList.Front(); e != nil; e = e.Next() {
		entry := e.Value.(*CacheEntry)
		item := map[string]any{
			"key":             entry.Key,
			"consecutiveHits": entry.ConsecutiveHits,
			"isBeingUsed":     entry.IsBeingUsed,
			"lastAccessed":    c.lastAccessTime[entry.Key],
		}
		status = append(status, item)
	}

	return status
}

// Clear removes all items from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.evictionList = list.New()
	c.lastAccessTime = make(map[string]time.Time)
}

// Get returns the value for a key if it exists in the cache
func (c *Cache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	element, exists := c.items[key]
	if !exists {
		return "", false
	}

	entry := element.Value.(*CacheEntry)
	return entry.Value, true
}

// Update updates an existing cache entry
func (c *Cache) Update(key string, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, exists := c.items[key]; exists {
		c.evictionList.MoveToFront(element)
		entry := element.Value.(*CacheEntry)
		entry.Value = value
		c.lastAccessTime[key] = time.Now()
	}
}

// GetLastAccessTime returns the last access time for a key
func (c *Cache) GetLastAccessTime(key string) (time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	lastAccess, exists := c.lastAccessTime[key]
	return lastAccess, exists
}

// GetConsecutiveHits returns the consecutive hits for a key
func (c *Cache) GetConsecutiveHits(key string) (int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	element, exists := c.items[key]
	if !exists {
		return 0, false
	}

	entry := element.Value.(*CacheEntry)
	return entry.ConsecutiveHits, true
}

// IncrementConsecutiveHits increments the consecutive hits for a key
func (c *Cache) IncrementConsecutiveHits(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, exists := c.items[key]; exists {
		entry := element.Value.(*CacheEntry)
		entry.ConsecutiveHits++
		if entry.ConsecutiveHits >= 2 {
			entry.IsBeingUsed = true
		}
	}
}

// ResetConsecutiveHits resets the consecutive hits for a key
func (c *Cache) ResetConsecutiveHits(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, exists := c.items[key]; exists {
		entry := element.Value.(*CacheEntry)
		entry.ConsecutiveHits = 1
		entry.IsBeingUsed = false
	}
}

// Len returns the current number of items in the cache
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.evictionList.Len()
}

// Size returns the maximum size of the cache
func (c *Cache) Size() int {
	return c.size
}
