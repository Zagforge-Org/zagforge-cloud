package contextcache_test

import (
	"context"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/cache/contextcache"
)

func TestKey(t *testing.T) {
	got := contextcache.Key("repo-123", "abc456")
	want := "ctx:repo-123:abc456"
	if got != want {
		t.Errorf("Key() = %q, want %q", got, want)
	}
}

func TestInMemoryCache_GetMiss(t *testing.T) {
	c := contextcache.NewInMemory()
	_, ok, err := c.Get(context.Background(), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected cache miss, got hit")
	}
}

func TestInMemoryCache_SetAndGet(t *testing.T) {
	c := contextcache.NewInMemory()
	ctx := context.Background()

	if err := c.Set(ctx, "k1", "value1"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	val, ok, err := c.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if val != "value1" {
		t.Errorf("got %q, want %q", val, "value1")
	}
}

func TestInMemoryCache_Overwrite(t *testing.T) {
	c := contextcache.NewInMemory()
	ctx := context.Background()

	c.Set(ctx, "k1", "old")
	c.Set(ctx, "k1", "new")

	val, ok, _ := c.Get(ctx, "k1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if val != "new" {
		t.Errorf("got %q, want %q", val, "new")
	}
}

func TestInMemoryCache_IsolatedKeys(t *testing.T) {
	c := contextcache.NewInMemory()
	ctx := context.Background()

	c.Set(ctx, "k1", "v1")
	c.Set(ctx, "k2", "v2")

	v1, _, _ := c.Get(ctx, "k1")
	v2, _, _ := c.Get(ctx, "k2")

	if v1 != "v1" || v2 != "v2" {
		t.Errorf("keys not isolated: k1=%q k2=%q", v1, v2)
	}
}

func TestInMemoryCache_ImplementsInterface(t *testing.T) {
	var _ contextcache.Cache = contextcache.NewInMemory()
}

func TestRedisCache_ImplementsInterface(t *testing.T) {
	var _ contextcache.Cache = contextcache.NewRedis(nil)
}
