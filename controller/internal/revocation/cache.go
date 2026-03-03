package revocation

import "sync"

// Cache provides hot-path revocation lookups for active mTLS connections.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]struct{}
}

func NewCache() *Cache {
	return &Cache{entries: map[string]struct{}{}}
}

func (c *Cache) Add(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = struct{}{}
}

func (c *Cache) Contains(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.entries[key]
	return ok
}
