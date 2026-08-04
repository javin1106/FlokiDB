package core

import (
	"javinkv/config"
	"time"
)

var store map[string]*Obj

type Obj struct {
	Value     interface{} // Redis SET key,value pairs are for all the data types not just string
	ExpiresAt int64
}

func init() {
	store = make(map[string]*Obj)
}

// If you passed the cache around without pointers, every function would get its own isolated copy of the database, completely defeating the purpose of a shared cache.

func NewObj(value interface{}, durationMs int64) *Obj {
	var expiresAt int64 = -1
	if durationMs > 0 {
		expiresAt = time.Now().UnixMilli() + durationMs
	}

	return &Obj{
		Value:     value,
		ExpiresAt: expiresAt,
	}
}

func Put(key string, obj *Obj) {
	if len(store) >= config.KeysLimit {
		evict()
	}
	store[key] = obj
}

func Get(k string) *Obj {
	return store[k]
}

func Del(k string) bool {
	if _, ok := store[k]; ok {
		delete(store, k)
		return true // delete was successful
	}
	return false // unsuccessful
}
