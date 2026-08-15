package main

import (
	"sync"
	"testing"
	"time"
)

func TestCreateTokenBucket(t *testing.T) {
	tb := createTokenBucket(5, 3, 1)
	defer tb.close()

	// Initially 5 tokens should be available.
	for i := 0; i < 5; i++ {
		if !tb.allow() {
			t.Fatalf("expected token %d to be allowed", i+1)
		}
	}

	// Bucket should now be empty.
	if tb.allow() {
		t.Fatal("expected request to be rejected when bucket is empty")
	}
}

func TestTokenBucketRefill(t *testing.T) {
	tb := createTokenBucket(5, 3, 1)
	defer tb.close()

	// Consume all 5 initial tokens.
	for i := 0; i < 5; i++ {
		if !tb.allow() {
			t.Fatalf("expected token %d to be allowed", i+1)
		}
	}

	// Bucket is empty.
	if tb.allow() {
		t.Fatal("expected bucket to be empty")
	}

	// Wait for one refill.
	time.Sleep(1100 * time.Millisecond)

	// 3 tokens should have been refilled.
	for i := 0; i < 3; i++ {
		if !tb.allow() {
			t.Fatalf("expected refilled token %d to be allowed", i+1)
		}
	}

	// Should be empty again.
	if tb.allow() {
		t.Fatal("expected bucket to be empty after consuming refill")
	}
}

func TestTokenBucketDoesNotExceedCapacity(t *testing.T) {
	tb := createTokenBucket(5, 10, 1)
	defer tb.close()

	// Wait for refill.
	time.Sleep(1100 * time.Millisecond)

	// Capacity is 5, so only 5 requests should be allowed.
	for i := 0; i < 5; i++ {
		if !tb.allow() {
			t.Fatalf("expected token %d to be allowed", i+1)
		}
	}

	if tb.allow() {
		t.Fatal("bucket exceeded its maximum capacity")
	}
}

func TestRateLimiterGetBucket(t *testing.T) {
	rl := CreateRateLimiter(5, 3, 1)

	bucket1 := rl.GetBucket("client-1")
	bucket2 := rl.GetBucket("client-1")

	if bucket1 != bucket2 {
		t.Fatal("expected GetBucket to return the same bucket for the same client")
	}

	bucket3 := rl.GetBucket("client-2")

	if bucket1 == bucket3 {
		t.Fatal("expected different clients to have different buckets")
	}

	bucket1.close()
	bucket3.close()
}

func TestRateLimiterRemoveClient(t *testing.T) {
	rl := CreateRateLimiter(5, 3, 1)

	bucket1 := rl.GetBucket("client-1")

	rl.RemoveClient("client-1")

	bucket2 := rl.GetBucket("client-1")

	if bucket1 == bucket2 {
		t.Fatal("expected a new bucket after removing the client")
	}

	bucket2.close()
}

func TestRateLimiterConcurrentGetBucket(t *testing.T) {
	rl := CreateRateLimiter(5, 3, 1)

	const goroutines = 100

	buckets := make([]Bucket, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			buckets[i] = rl.GetBucket("same-client")
		}()
	}

	wg.Wait()

	// Every goroutine should get the exact same bucket.
	for i := 1; i < goroutines; i++ {
		if buckets[i] != buckets[0] {
			t.Fatal("concurrent GetBucket returned different buckets")
		}
	}

	buckets[0].close()
}

func TestTokenBucketConcurrentAllow(t *testing.T) {
	tb := createTokenBucket(100, 0, 1)
	defer tb.close()

	const goroutines = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)

	var allowed int
	var mu sync.Mutex

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			if tb.allow() {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if allowed != 100 {
		t.Fatalf("expected exactly 100 allowed requests, got %d", allowed)
	}
}

func TestRateLimiterConcurrentClients(t *testing.T) {
	rl := CreateRateLimiter(100, 0, 1)

	const clients = 100
	const requestsPerClient = 100

	var wg sync.WaitGroup
	wg.Add(clients)

	for i := 0; i < clients; i++ {
		client := i

		go func() {
			defer wg.Done()

			clientName := string(rune('A' + client))

			bucket := rl.GetBucket(clientName)

			for j := 0; j < requestsPerClient; j++ {
				bucket.allow()
			}
		}()
	}

	wg.Wait()

	// The test is primarily checking that concurrent access does not
	// cause a race or panic.
}

func TestTokenBucketClose(t *testing.T) {
	tb := createTokenBucket(5, 3, 1)

	// Closing should signal the refill goroutine to stop.
	tb.close()

	// Give the goroutine a moment to exit.
	time.Sleep(50 * time.Millisecond)
}
