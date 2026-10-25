package main

import (
	"container/list"
	"sync"
)

type LRU[K comparable, V any] interface {
	Get(key K) (value V, ok bool)
	Set(key K, value V)
}
type entry[K comparable, V any] struct {
	key   K
	value V
}
type LRUCache[K comparable, V any] struct {
	capacity int
	mu       sync.Mutex
	ll       list.List
	items    map[K]*list.Element
}

func NewLRUCache[K comparable, V any](capacity int) *LRUCache[K, V] {
	return &LRUCache[K, V]{
		capacity: capacity,
		items: make(map[K]*list.Element),
	}
}
func (c *LRUCache[K, V]) Get(key K) (value V, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	elem, ok := c.items[key]
	if !ok {
		return
	}
	c.ll.MoveToFront(elem)
	rec := elem.Value.(*entry[K, V])
	value = rec.value
	return
}
func (c *LRUCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.capacity <= 0 {
		return
	}
	elem, exist := c.items[key]
	if exist {
		rec := elem.Value.(*entry[K, V])
		rec.value = value
		return
	}
	if len(c.items) >= c.capacity {
		old := c.ll.Back()
		delete(c.items, old.Value.(*entry[K, V]).key)
		c.ll.Remove(old)
	}
	c.ll.PushFront(&entry[K, V]{key, value})
	c.items[key] = c.ll.Front()
	return
}
