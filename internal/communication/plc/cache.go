package plc

import (
	"container/list"
	"sync"
	"time"
)

// CachedValue representa un valor en cache con metadatos
type CachedValue struct {
	Value     interface{}
	Timestamp time.Time
}

// LRUCache implementa una cache LRU O(1) thread-safe
type LRUCache struct {
	maxEntries int
	ttl        time.Duration
	entries    map[string]*list.Element
	lruList    *list.List
	mu         sync.RWMutex
}

// entry es un elemento interno de la cache
type entry struct {
	key   string
	value *CachedValue
}

// NewLRUCache crea una nueva cache LRU
func NewLRUCache(maxEntries int, ttl time.Duration) *LRUCache {
	return &LRUCache{
		maxEntries: maxEntries,
		ttl:        ttl,
		entries:    make(map[string]*list.Element),
		lruList:    list.New(),
	}
}

// Get obtiene un valor de la cache
// Retorna (valor, encontrado)
func (c *LRUCache) Get(key string) (*CachedValue, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	entry := elem.Value.(*entry)

	// Verificar TTL
	if time.Since(entry.value.Timestamp) > c.ttl {
		// Expirado - eliminar
		c.removeElement(elem)
		return nil, false
	}

	// Mover al frente (más recientemente usado)
	c.lruList.MoveToFront(elem)
	return entry.value, true
}

// Set almacena un valor en la cache
func (c *LRUCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Si ya existe, actualizar y mover al frente
	if elem, ok := c.entries[key]; ok {
		c.lruList.MoveToFront(elem)
		entry := elem.Value.(*entry)
		entry.value = &CachedValue{
			Value:     value,
			Timestamp: time.Now(),
		}
		return
	}

	// Nueva entrada
	newEntry := &entry{
		key: key,
		value: &CachedValue{
			Value:     value,
			Timestamp: time.Now(),
		},
	}

	elem := c.lruList.PushFront(newEntry)
	c.entries[key] = elem

	// Evictar si excede tamaño máximo
	if c.lruList.Len() > c.maxEntries {
		c.evictOldest()
	}
}

// evictOldest elimina el elemento menos recientemente usado (al final de la lista)
func (c *LRUCache) evictOldest() {
	elem := c.lruList.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

// removeElement elimina un elemento de la cache
func (c *LRUCache) removeElement(elem *list.Element) {
	c.lruList.Remove(elem)
	entry := elem.Value.(*entry)
	delete(c.entries, entry.key)
}

// Clear limpia toda la cache
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*list.Element)
	c.lruList = list.New()
}

// Len retorna el número de entradas en la cache
func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lruList.Len()
}
