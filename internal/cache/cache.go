// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package cache implements an inmemory cache for any interface{} object.
package cache

import (
	"errors"
	"sync"
	"time"
)

var ErrInvalidDuration = errors.New("expireAfter duration cannot be negative")

type WriteThru func() (interface{}, error)

type Cache struct {
	data map[string]item
	mu   sync.RWMutex
}

type item struct {
	object    interface{}
	expiresAt int64
}

func (i *item) expired() bool {
	return i.expiresAt < time.Now().UnixNano()
}

// New creates a new in memory cache.
func New() *Cache {
	return &Cache{
		data: make(map[string]item),
	}
}

func (c *Cache) purgeExpired(name string, exp int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, ok := c.data[name]; ok && item.expiresAt == exp {
		// found, and the expiry time is still the same as when the purge was requested.
		delete(c.data, name)
	}
}

// Size returns the number of items in the cache.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

func (c *Cache) WriteThruLookup(name string, funk WriteThru, expireAfter time.Duration) (interface{}, error) {
	if expireAfter < 0 {
		return nil, ErrInvalidDuration
	}

	// This call takes a read lock.
	val, hit := c.Lookup(name)
	if hit {
		return val, nil
	}

	// Escalate the lock to a RW lock.
	c.mu.Lock()
	defer c.mu.Unlock()
	// double check that the value hasn't been set by another goroutine.
	if val, hit := c.data[name]; hit && !val.expired() {
		return val, nil
	}
	// Either a miss, or hit w/ expired value.

	// Value does indeed need to be refreshed. Used the provided fucntion.
	newData, err := funk()
	if err != nil {
		return nil, err
	}

	// save the newData in the cache. newData may be nil, if that's what the WriteThruFunction provided.
	c.data[name] = item{
		object:    newData,
		expiresAt: time.Now().Add(expireAfter).UnixNano(),
	}
	return newData, nil

}

// Lookup checks the cache for a non-expired object by the supplied key name.
// The bool return informs the caller if there was a cache hit or not.
// A return of nil, true means that nil is in the cache.
// Where nil, false indicates a cache miss or that the value is expired and should
// be refreshed.
func (c *Cache) Lookup(name string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if item, ok := c.data[name]; ok && item.expired() {
		// Cache hit, but expired. The removal from the cache is deferred.
		go c.purgeExpired(name, item.expiresAt)
		return nil, false
	} else if ok {
		// Cache hit, not expired.
		return item.object, true
	}
	// Cache miss.
	return nil, false
}

// Set saves the current value of an object in the cache, with the supplied
// durintion until the object expires.
func (c *Cache) Set(name string, object interface{}, expireAfter time.Duration) error {
	if expireAfter < 0 {
		return ErrInvalidDuration
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[name] = item{
		object:    object,
		expiresAt: time.Now().Add(expireAfter).UnixNano(),
	}

	return nil
}
