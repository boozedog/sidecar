package ui

import (
	"hash/maphash"
	"sync"
	"sync/atomic"

	"github.com/charmbracelet/x/ansi"
)

// TruncateCache provides cached ANSI-aware truncation to eliminate allocation churn.
// Thread-safe for concurrent access from rendering goroutines.
type TruncateCache struct {
	mu       sync.RWMutex
	entries  map[cacheKey]string
	maxSize  int
	hashSeed maphash.Seed
	hits     atomic.Int64
	misses   atomic.Int64
	clears   atomic.Int64
}

// cacheKey uniquely identifies a truncation operation using content hash.
type cacheKey struct {
	hash   uint64 // Hash of content
	length int    // Length of content (collision guard)
	width  int
	offset int  // For TruncateLeft (0 for Truncate)
	isLeft bool // true for TruncateLeft, false for Truncate
}

// NewTruncateCache creates a new truncation cache with the given maximum size.
// maxSize limits memory growth; when exceeded, the cache is cleared.
func NewTruncateCache(maxSize int) *TruncateCache {
	return &TruncateCache{
		entries:  make(map[cacheKey]string, maxSize),
		maxSize:  maxSize,
		hashSeed: maphash.MakeSeed(),
	}
}

// Truncate returns the content truncated to width using ANSI-aware truncation.
// Results are cached to avoid repeated parser allocations.
func (c *TruncateCache) Truncate(content string, width int, tail string) string {
	if c == nil || width <= 0 {
		return content
	}

	// Hash content instead of storing it directly
	hash := maphash.String(c.hashSeed, content)
	key := cacheKey{
		hash:   hash,
		length: len(content),
		width:  width,
		offset: 0,
		isLeft: false,
	}

	// Check cache (read lock)
	c.mu.RLock()
	if result, ok := c.entries[key]; ok {
		c.mu.RUnlock()
		c.hits.Add(1)
		c.maybeLogStats()
		return result
	}
	c.mu.RUnlock()

	// Cache miss - compute result
	c.misses.Add(1)
	result := ansi.Truncate(content, width, tail)

	// Store in cache (write lock)
	c.mu.Lock()
	// Check size limit before inserting
	if len(c.entries) >= c.maxSize {
		// Clear cache when full to prevent unbounded growth
		c.entries = make(map[cacheKey]string, c.maxSize)
	}
	c.entries[key] = result
	c.mu.Unlock()

	c.maybeLogStats()
	return result
}

// TruncateLeft returns the content truncated from the left to width using ANSI-aware truncation.
// Results are cached to avoid repeated parser allocations.
func (c *TruncateCache) TruncateLeft(content string, offset int, tail string) string {
	if c == nil || offset <= 0 {
		return content
	}

	// Hash content instead of storing it directly
	hash := maphash.String(c.hashSeed, content)
	key := cacheKey{
		hash:   hash,
		length: len(content),
		width:  0,
		offset: offset,
		isLeft: true,
	}

	// Check cache (read lock)
	c.mu.RLock()
	if result, ok := c.entries[key]; ok {
		c.mu.RUnlock()
		c.hits.Add(1)
		c.maybeLogStats()
		return result
	}
	c.mu.RUnlock()

	// Cache miss - compute result
	c.misses.Add(1)
	result := ansi.TruncateLeft(content, offset, tail)

	// Store in cache (write lock)
	c.mu.Lock()
	// Check size limit before inserting
	if len(c.entries) >= c.maxSize {
		// Clear cache when full to prevent unbounded growth
		c.entries = make(map[cacheKey]string, c.maxSize)
	}
	c.entries[key] = result
	c.mu.Unlock()

	c.maybeLogStats()
	return result
}

// Clear removes all cached entries.
// Should be called when window resizes to avoid stale results.
func (c *TruncateCache) Clear() {
	if c == nil {
		return
	}
	c.clears.Add(1)
	c.mu.Lock()
	c.entries = make(map[cacheKey]string, c.maxSize)
	c.mu.Unlock()
}

// Size returns the current number of cached entries (for testing/monitoring).
func (c *TruncateCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// maybeLogStats is a no-op; cached counters (hits, misses, clears) can be
// inspected directly for profiling purposes.
func (c *TruncateCache) maybeLogStats() {
	// Stats counters available: c.hits, c.misses, c.clears (atomic.Int64)
}
