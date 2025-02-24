package ttl

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGet(t *testing.T) {
	cache := NewCache[string, string](time.Second)

	data := cache.Get("hello")
	assert.Equal(t, "", data)

	cache.Set("hello", "world")
	data = cache.Get("hello")
	assert.Equal(t, "world", data)
}

func TestGetOrSet(t *testing.T) {
	cache := NewCache[string, string](time.Second)

	data, _ := cache.GetOrSet("hello", "earth")
	assert.Equal(t, "earth", data)

	cache.GetOrSet("hello", "world")
	data = cache.Get("hello")
	assert.Equal(t, "earth", data)
}

func TestGetAndDelete(t *testing.T) {
	cache := NewCache[string, string](time.Second)

	data, _ := cache.GetOrSet("hello", "earth")
	assert.Equal(t, "earth", data)

	data, _ = cache.GetAndDelete("hello")
	assert.Equal(t, "earth", data)
	data = cache.Get("hello")
	assert.Equal(t, "", data)
}

func TestExpiration(t *testing.T) {
	cache := NewCache[string, string](time.Second)

	cache.Set("x", "1")
	cache.Set("y", "z")
	cache.Set("z", "3")

	<-time.After(500 * time.Millisecond)
	val := cache.Get("x")
	assert.Equal(t, "1", val)

	<-time.After(time.Second)
	val = cache.Get("x")
	assert.Equal(t, "", val)
	val = cache.Get("y")
	assert.Equal(t, "", val)
	val = cache.Get("z")
	assert.Equal(t, "", val)
	assert.Equal(t, 0, len(cache.items))
}
