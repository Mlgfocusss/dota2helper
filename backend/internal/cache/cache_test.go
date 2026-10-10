package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTTLExpiry(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	c := New[int](10, time.Hour)
	c.now = func() time.Time { return now }

	c.Set("a", 1, time.Minute)
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("Get = %v, %v; ожидалось 1, true", v, ok)
	}
	now = now.Add(2 * time.Minute)
	if _, ok := c.Get("a"); ok {
		t.Fatal("просроченное значение не должно отдаваться через Get")
	}
}

func TestGetOrLoadDeduplicates(t *testing.T) {
	c := New[string](10, 0)
	var calls atomic.Int32
	release := make(chan struct{})
	load := func(context.Context) (string, error) {
		calls.Add(1)
		<-release
		return "v", nil
	}

	var wg sync.WaitGroup
	results := make([]string, 5)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := c.GetOrLoad(context.Background(), "k", time.Minute, load)
			if err != nil {
				t.Error(err)
			}
			results[i] = v
		}()
	}
	time.Sleep(50 * time.Millisecond) // все горутины ждут одну загрузку
	close(release)
	wg.Wait()

	if n := calls.Load(); n != 1 {
		t.Errorf("load вызван %d раз, ожидался 1", n)
	}
	for _, v := range results {
		if v != "v" {
			t.Errorf("результат %q, ожидался %q", v, "v")
		}
	}
	if v, ok := c.Get("k"); !ok || v != "v" {
		t.Error("значение должно сохраниться в кэше")
	}
}

func TestGetOrLoadServesStaleOnError(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	c := New[string](10, time.Hour)
	c.now = func() time.Time { return now }
	c.Set("k", "old", time.Minute)
	now = now.Add(5 * time.Minute)

	failing := func(context.Context) (string, error) { return "", errors.New("api down") }
	v, err := c.GetOrLoad(context.Background(), "k", time.Minute, failing)
	if err != nil || v != "old" {
		t.Fatalf("ожидалось устаревшее значение, получено %q, %v", v, err)
	}

	now = now.Add(2 * time.Hour) // за пределами staleFor
	if _, err := c.GetOrLoad(context.Background(), "k", time.Minute, failing); err == nil {
		t.Fatal("слишком старое значение отдавать нельзя — ожидалась ошибка")
	}
}

func TestEviction(t *testing.T) {
	c := New[int](3, 0)
	for i, k := range []string{"a", "b", "c", "d"} {
		c.Set(k, i, time.Minute)
	}
	if len(c.items) > 3 {
		t.Errorf("в кэше %d ключей, максимум 3", len(c.items))
	}
}
