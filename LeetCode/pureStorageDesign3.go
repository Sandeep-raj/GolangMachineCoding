package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

/*
Approach	Thread safety	Throughput	Complexity	Distributed
Mutex Snowflake	Mutex	High	Low	Yes
CAS Snowflake	Atomic CAS	Very high	Medium	Yes
Range allocation	Atomic/local	Extremely high	Medium	Yes
Pre-generated pool	Channel/atomic	Very high	Medium	Needs allocator
*/

type BlockGenerator struct {
	next      atomic.Int64
	mu        *sync.Mutex
	end       int64
	blockSize int64
}

func createBlockGenerator(start, blkSize int64) *BlockGenerator {
	gen := BlockGenerator{
		blockSize: blkSize,
		mu:        &sync.Mutex{},
	}
	gen.next.Store(start)
	gen.end = start + blkSize
	return &gen
}

func (gen *BlockGenerator) allocateBlock() {
	// In production:
	// atomically reserve a block from DB / ID service.
	gen.next.Store(gen.end)
	gen.end = gen.end + gen.blockSize
}

func (gen *BlockGenerator) NextID() int64 {
	gen.mu.Lock()
	if gen.next.Load() == gen.end {
		gen.allocateBlock()
	}
	gen.mu.Unlock()

	val := gen.next.Add(1)
	return val
}

func main() {
	wg := sync.WaitGroup{}
	gen := createBlockGenerator(100000, 1000)

	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			fmt.Println(gen.NextID())
		}()
	}

	wg.Wait()
	fmt.Println("completed all the goroutines")
}
