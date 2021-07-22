// Copyright 2020 the Exposure Notifications Server authors
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
//
// Although exported, this package is non intended for general consumption.
// It is a shared dependency between multiple exposure notifications projects.
// We cannot guarantee that there won't be breaking changes in the future.
package cache

import (
	"errors"
	"sync"
	"time"
)

var ErrInvalidDuration = errors.New("expireAfter duration cannot be negative")

const initialSize = 16

type Func func() (interface{}, error)

type Cache struct {
	data        map[string]item
	expireAfter time.Duration
	mu          sync.RWMutex
}

type item struct {
	object    interface{}
	expiresAt int64
}

func (i *item) expired() bool {
	return i.expiresAt < time.Now().UnixNano()
}

// New creates a new in memory cache.
func New(expireAfter time.Duration) (*Cache, error) {
	if expireAfter < 0 {
		return nil, ErrInvalidDuration
	}

	return &Cache{
		data:        make(map[string]item, initialSize),
		expireAfter: expireAfter,
	}, nil
}

// Removes an item by name and expiry time when the purge was scheduled.
// If there is a race, and the item has been refreshed, it will not be purged.
func (c *Cache) purgeExpired(name string, expectedExpiryTime int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, ok := c.data[name]; ok && item.expiresAt == expectedExpiryTime {
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

// Clear removes all items from the cache, regardless of their expiration.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]item, initialSize)
}

// WriteThruLookup checks the cache for the value associated with name,
// and if not found or expired, invokes the provided primaryLookup function
// to local the value.
func (c *Cache) WriteThruLookup(name string, primaryLookup Func) (interface{}, error) {
	c.mu.RLock()
	val, hit := c.lookup(name)
	if hit {
		c.mu.RUnlock()
		return val, nil
	}
	c.mu.RUnlock()

	// Ensure the value hasn't been set by another goroutine by escalating to a RW
	// lock. We need the W lock anyway if we're about to write.
	c.mu.Lock()
	defer c.mu.Unlock()
	val, hit = c.lookup(name)
	if hit {
		return val, nil
	}

	// If we got this far, it was either a miss, or hit w/ expired value, execute
	// the function.

	// Value does indeed need to be refreshed. Used the provided function.
	newData, err := primaryLookup()
	if err != nil {
		return nil, err
	}

	// save the newData in the cache. newData may be nil, if that's what the WriteThruFunction provided.
	c.data[name] = item{
		object:    newData,
		expiresAt: time.Now().Add(c.expireAfter).UnixNano(),
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

	return c.lookup(name)
}

// Set saves the current value of an object in the cache, with the supplied
// durintion until the object expires.
func (c *Cache) Set(name string, object interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[name] = item{
		object:    object,
		expiresAt: time.Now().Add(c.expireAfter).UnixNano(),
	}

	return nil
}

// lookup finds an unexpired item at the given name. The bool indicates if a hit
// occurred. This is an internal API that is NOT thread-safe. Consumers must
// take out a read or read-write lock.
func (c *Cache) lookup(name string) (interface{}, bool) {
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
