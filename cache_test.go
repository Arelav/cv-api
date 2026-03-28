package main

import (
	"testing"
	"time"
)

func TestCache_MissOnEmpty(t *testing.T) {
	c := newCache[string](time.Hour)
	_, ok := c.get()
	if ok {
		t.Fatal("expected cache miss on empty cache")
	}
}

func TestCache_HitAfterSet(t *testing.T) {
	c := newCache[string](time.Hour)
	c.set("hello")
	v, ok := c.get()
	if !ok {
		t.Fatal("expected cache hit")
	}
	if v != "hello" {
		t.Fatalf("got %q, want %q", v, "hello")
	}
}

func TestCache_Expiry(t *testing.T) {
	c := newCache[string](time.Millisecond)
	c.set("hello")
	time.Sleep(5 * time.Millisecond)
	_, ok := c.get()
	if ok {
		t.Fatal("expected cache miss after TTL")
	}
}
