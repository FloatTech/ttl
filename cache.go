package ttl

import (
	"sync"
	"time"
)

// Cache is a synchronised map of items that auto-expire once stale
type Cache[K comparable, V any] struct {
	mu    sync.RWMutex
	ttl   time.Duration
	items map[K]*Item[V]
	onset func(K, V)
	onget func(K, V)
	ondel func(K, V)
	ontch func(K, V)
	stop  func() // Stop stops the gc loop
}

// NewCache 创建指定生命周期的 Cache
func NewCache[K comparable, V any](ttl time.Duration) *Cache[K, V] {
	return NewCacheOn(ttl, [4]func(K, V){})
}

// NewCacheOn 创建指定生命周期的 Cache
//
//	on: [onset, onget, ondel, ontouch]
func NewCacheOn[K comparable, V any](ttl time.Duration, on [4]func(K, V)) *Cache[K, V] {
	cache := &Cache[K, V]{
		ttl:   ttl,
		items: map[K]*Item[V]{},
		onset: on[0],
		onget: on[1],
		ondel: on[2],
		ontch: on[3],
	}
	cache.stop = cache.gc() // async gc
	return cache
}

func (c *Cache[K, V]) gc() (stop func()) {
	ticker := time.NewTicker(time.Minute)
	stopchan := make(chan struct{})
	go func() {
	loop:
		for {
			select {
			case <-stopchan:
				break loop
			case <-ticker.C:
				c.mu.Lock()
				if c.items == nil || c.stop == nil {
					break loop
				}
				for key, item := range c.items {
					if item.expired() {
						if c.ondel != nil {
							c.ondel(key, c.items[key].value)
						}
						delete(c.items, key)
					}
				}
				c.mu.Unlock()
			}
		}
	}()
	return func() {
		ticker.Stop()
		stopchan <- struct{}{}
	}
}

// Destroy 销毁 chahe, 不可再使用, 否则 panic
func (c *Cache[K, V]) Destroy() {
	c.stop()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ondel != nil {
		for k, v := range c.items {
			c.ondel(k, v.value)
		}
	}
	c.items = nil
	c.stop = nil
}

// get 无锁通过 key 获取指定的元素
func (c *Cache[K, V]) get(key K, item *Item[V]) (v V) {
	if item.expired() {
		c.delete(key, item)
		return
	}
	item.exp = time.Now().Add(c.ttl) // reset the expired time
	if c.onget != nil {
		c.onget(key, item.value)
	}
	return item.value
}

// Get 通过 key 获取指定的元素
func (c *Cache[K, V]) Get(key K) (v V) {
	c.mu.RLock()
	item := c.items[key]
	c.mu.RUnlock()
	if item == nil {
		return
	}
	return c.get(key, item)
}

// GetOrSet 获取 key 的值或在为空时设置 key 的值并返回该值
func (c *Cache[K, V]) GetOrSet(key K, val V) (v V, got bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item := c.items[key]
	if item != nil {
		return c.get(key, item), true
	}
	v = val
	c.set(key, &Item[V]{
		exp:   time.Now().Add(c.ttl),
		value: val,
	})
	return
}

// set 无锁设置指定 key 的值
func (c *Cache[K, V]) set(key K, item *Item[V]) {
	c.items[key] = item
	if c.onset != nil {
		c.onset(key, item.value)
	}
}

// Set 设置指定 key 的值
func (c *Cache[K, V]) Set(key K, val V) {
	item := &Item[V]{
		exp:   time.Now().Add(c.ttl),
		value: val,
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.set(key, item)
}

// delete 无锁删除 item
func (c *Cache[K, V]) delete(key K, item *Item[V]) {
	if c.ondel != nil {
		c.ondel(key, item.value)
	}
	delete(c.items, key)
}

// Delete 获得值后删除指定key
func (c *Cache[K, V]) GetAndDelete(key K) (v V, deleted bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item := c.items[key]
	if item == nil { // no such key
		return
	}
	c.delete(key, item)
	return item.value, true
}

// Delete 删除指定key
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item := c.items[key]
	if item == nil { // no such key
		return
	}
	c.delete(key, item)
}

// Touch 为指定key添加一定生命周期
func (c *Cache[K, V]) Touch(key K, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items[key] != nil {
		c.items[key].exp = c.items[key].exp.Add(ttl)
		if c.ontch != nil {
			c.ontch(key, c.items[key].value)
		}
	}
}

// get without readlock & expired delete
func (c *Cache[K, V]) rangeget(key K) (v V) {
	item := c.items[key]
	if item == nil || item.expired() {
		return
	}
	item.exp = time.Now().Add(c.ttl) // reset the expired time
	if c.onget != nil {
		c.onget(key, item.value)
	}
	return item.value
}

func (c *Cache[K, V]) Range(f func(K, V) error) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for k := range c.items {
		err := f(k, c.rangeget(k))
		if err != nil {
			return err
		}
	}
	return nil
}
