package main

import (
	"testing"
	"time"
)

func TestKeyExpiration(t *testing.T) {
	cache := CreateCache()
	cache.set("test", "test", 6000)

	time.Sleep(6 * time.Second)
	_, err := cache.get("test")

	if err == nil {
		t.FailNow()
	}
}
