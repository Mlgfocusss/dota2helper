// Package cache — in‑process кэш с TTL. Параллельные загрузки одного ключа
// схлопываются в одну, а при ошибке загрузки можно отдать устаревшее значение.
package cache

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type entry[V any] struct {
	value   V
	expires time.Time
}

type TTL[V any] struct {
	mu       sync.Mutex
	items    map[string]entry[V]
	group    singleflight.Group
	maxSize  int
	staleFor time.Duration
	now      func() time.Time
}

// New создаёт кэш на maxSize ключей. staleFor — сколько после истечения TTL
// значение ещё можно отдать, если свежие данные загрузить не удалось.
func New[V any](maxSize int, staleFor time.Duration) *TTL[V] {
	return &TTL[V]{
		items:    make(map[string]entry[V]),
		maxSize:  maxSize,
		staleFor: staleFor,
		now:      time.Now,
	}
}

// Get возвращает значение, только если оно не просрочено.
func (c *TTL[V]) Get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[key]
	if !ok || !c.now().Before(e.expires) {
		var zero V
		return zero, false
	}
	return e.value, true
}

func (c *TTL[V]) Set(key string, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = entry[V]{value: value, expires: c.now().Add(ttl)}
	if len(c.items) > c.maxSize {
		c.evictLocked()
	}
}

func (c *TTL[V]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// GetOrLoad отдаёт свежее значение из кэша или вызывает load — один раз на ключ,
// даже если значение одновременно запросили несколько горутин. Отмена ctx одного
// вызывающего не прерывает общую загрузку. Если load вернул ошибку, а устаревшее
// значение ещё в пределах staleFor, возвращается оно.
func (c *TTL[V]) GetOrLoad(ctx context.Context, key string, ttl time.Duration, load func(context.Context) (V, error)) (V, error) {
	if v, ok := c.Get(key); ok {
		return v, nil
	}
	ch := c.group.DoChan(key, func() (any, error) {
		v, err := load(context.WithoutCancel(ctx))
		if err == nil {
			c.Set(key, v, ttl)
		}
		return v, err
	})

	var zero V
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			if v, ok := c.stale(key); ok {
				return v, nil
			}
			return zero, res.Err
		}
		return res.Val.(V), nil
	}
}

func (c *TTL[V]) stale(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[key]
	if !ok || c.now().After(e.expires.Add(c.staleFor)) {
		var zero V
		return zero, false
	}
	return e.value, true
}

// evictLocked удаляет значения, которые уже нельзя отдать даже как устаревшие,
// а если места всё равно нет — произвольную десятую часть ключей.
func (c *TTL[V]) evictLocked() {
	now := c.now()
	for k, e := range c.items {
		if now.After(e.expires.Add(c.staleFor)) {
			delete(c.items, k)
		}
	}
	if len(c.items) <= c.maxSize {
		return
	}
	drop := len(c.items) / 10
	if drop == 0 {
		drop = 1
	}
	for k := range c.items {
		if drop == 0 {
			break
		}
		delete(c.items, k)
		drop--
	}
}
