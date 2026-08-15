package main

import (
	"errors"
	"log"
	"sync"
	"time"
)

type CacheValue struct {
	value string
	expt  int64
}

type Cache interface {
	set(k, val string, ttl int64)
	get(k string) (string, error)
	getttl(k string) (int64, error)
	expireKey(k string)
	del(k string) bool
}

type CacheImpl struct {
	m           *sync.Mutex
	entries     map[string]*CacheValue
	cacheExpiry *CacheExpiry
}

func (c *CacheImpl) set(k, val string, ttl int64) {
	c.m.Lock()
	defer c.m.Unlock()

	cv := &CacheValue{
		value: val,
		expt:  -1,
	}

	if ttl > 0 {
		cv.expt = time.Now().Add(time.Duration(ttl) * time.Millisecond).UnixMilli()
		c.cacheExpiry.updateChan <- KeyUpdateEvent{
			key:   k,
			value: val,
			expAt: cv.expt,
		}
	}

	c.entries[k] = cv
}

func (c *CacheImpl) get(k string) (string, error) {
	c.m.Lock()
	defer c.m.Unlock()

	currt := time.Now().UnixMilli()

	cv := c.entries[k]
	if cv == nil {
		return "", errors.New("no such key found")
	}

	if cv.expt != -1 && cv.expt < currt {
		delete(c.entries, k)
		return "", errors.New("no such key found")
	}

	return cv.value, nil
}

func (c *CacheImpl) del(k string) bool {
	c.m.Lock()
	defer c.m.Unlock()

	cv := c.entries[k]

	if cv != nil {
		delete(c.entries, k)
		return true
	}
	return false
}

func (c *CacheImpl) expireKey(k string) {
	c.m.Lock()
	defer c.m.Unlock()

	currt := time.Now().UnixMilli()
	cv := c.entries[k]

	if cv != nil && cv.expt != -1 && cv.expt <= currt {
		delete(c.entries, k)
	}
}

func (c *CacheImpl) getttl(k string) (int64, error) {
	c.m.Lock()
	defer c.m.Unlock()

	currt := time.Now().UnixMilli()

	cv := c.entries[k]
	if cv == nil {
		return 0, errors.New("no such key found")
	}

	if cv.expt != -1 && cv.expt < currt {
		delete(c.entries, k)
		return 0, errors.New("no such key found")
	}

	return (cv.expt - currt), nil
}

func CreateCache() Cache {
	c := &CacheImpl{
		m:       &sync.Mutex{},
		entries: make(map[string]*CacheValue),
	}
	ce := createCacheExpiry(2000, c)
	c.cacheExpiry = ce

	return c
}

type KeyUpdateEvent struct {
	key   string
	value string
	expAt int64
}

type CacheExpiry struct {
	m             *sync.Mutex
	keyList       map[string]int64
	checkDuration int64
	closeChan     chan struct{}
	updateChan    chan KeyUpdateEvent
	c             Cache
}

func createCacheExpiry(dur int64, c Cache) *CacheExpiry {
	if dur == 0 {
		dur = 2000
	}

	ce := &CacheExpiry{
		m:             &sync.Mutex{},
		keyList:       make(map[string]int64),
		checkDuration: dur,
		closeChan:     make(chan struct{}),
		updateChan:    make(chan KeyUpdateEvent, 1000),
		c:             c,
	}

	go ce.checkExpiry()
	go ce.RecvUpdateEvt()

	return ce
}

func (ce *CacheExpiry) RecvUpdateEvt() {
	for {
		select {
		case evt := <-ce.updateChan:
			ce.m.Lock()
			ce.keyList[evt.key] = evt.expAt
			ce.m.Unlock()
		case <-ce.closeChan:
			return
		}
	}
}

func (ce *CacheExpiry) checkExpiry() {
	ticker := time.NewTicker(time.Duration(ce.checkDuration) * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			ce.m.Lock()
			currt := time.Now().UnixMilli()
			for k, v := range ce.keyList {
				if v-currt < ce.checkDuration {
					delete(ce.keyList, k)
					go func(k string, delay int64) {
						time.Sleep(time.Duration(delay) * time.Millisecond)
						ce.c.expireKey(k)
					}(k, v-currt)
				}
			}
			ce.m.Unlock()
		case <-ce.closeChan:
			ticker.Stop()
			return
		}
	}
}

func main() {
	cache := CreateCache()
	cache.set("test", "test", 4000)
	cache.set("test1", "test", 4000)
	cache.set("test2", "test", 2000)
	cache.set("test3", "test", 6000)
	cache.set("test4", "test", 1000)
	cache.set("test5", "test", 1800)

	time.Sleep(2 * time.Second)
	log.Print(cache.get("test2"))
	log.Print(cache.get("test3"))
}
