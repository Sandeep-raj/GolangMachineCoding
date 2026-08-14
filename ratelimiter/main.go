package main

import (
	"log"
	"sync"
	"time"
)

type Bucket interface {
	allow() bool
	close()
}

type Tokenbucket struct {
	counter       int
	initToken     int
	refilToken    int
	refilDuration int
	closeChan     chan struct{}
	m             *sync.Mutex
}

func (tb *Tokenbucket) close() {
	close(tb.closeChan)
}

func (tb *Tokenbucket) allow() bool {
	tb.m.Lock()
	defer tb.m.Unlock()

	if tb.counter > 0 {
		tb.counter--
		return true
	}

	return false
}

func (tb *Tokenbucket) refill() {
	ticker := time.NewTicker(time.Duration(tb.refilDuration) * time.Second)

	for {
		select {
		case <-ticker.C:
			tb.m.Lock()
			tb.counter = tb.counter + tb.refilToken
			if tb.counter > tb.initToken {
				tb.counter = tb.initToken
			}
			tb.m.Unlock()
		case <-tb.closeChan:
			ticker.Stop()
			return
		}
	}
}

func createTokenBucket(initToken, refilCount, refilDuration int) *Tokenbucket {
	tokenBucket := Tokenbucket{
		counter:       initToken,
		initToken:     initToken,
		refilToken:    refilCount,
		refilDuration: refilDuration,
		closeChan:     make(chan struct{}),
		m:             &sync.Mutex{},
	}

	go tokenBucket.refill()

	return &tokenBucket
}

type RateLimiter struct {
	m       *sync.Mutex
	clients map[string]Bucket

	initToken     int
	refilCount    int
	refilDuration int
}

func CreateRateLimiter(initToken, refilCount, refilDuration int) *RateLimiter {
	return &RateLimiter{
		initToken:     initToken,
		refilCount:    refilCount,
		refilDuration: refilDuration,
		m:             &sync.Mutex{},
		clients:       make(map[string]Bucket),
	}
}

func (rl *RateLimiter) GetBucket(client string) Bucket {
	rl.m.Lock()
	defer rl.m.Unlock()

	if rl.clients[client] == nil {
		rl.clients[client] = createTokenBucket(rl.initToken, rl.refilCount, rl.refilDuration)
	}

	return rl.clients[client]
}

func (rl *RateLimiter) RemoveClient(client string) {
	rl.m.Lock()
	defer rl.m.Unlock()

	bucket := rl.clients[client]
	bucket.close()

	delete(rl.clients, client)
}

func testBucket(client string, rl *RateLimiter) {
	bucket := rl.GetBucket(client)
	for i := 0; i < 19; i++ {
		if !bucket.allow() {
			log.Printf("%s is not allowed", client)
		}
		time.Sleep(200 * time.Millisecond)
	}

	rl.RemoveClient(client)
}

func main() {
	ratelimiter := CreateRateLimiter(5, 3, 1)

	go testBucket("test1", ratelimiter)
	go testBucket("test2", ratelimiter)
	go testBucket("test3", ratelimiter)

	time.Sleep(10 * time.Second)
}
