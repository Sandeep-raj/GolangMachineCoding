package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// 1. Naive implementation: generate/check one ID at a time.
// ============================================================
// Interview note:
// - Simple, but collision checking requires state.
// - With random IDs, collision probability eventually grows.
// - A mutex is required if multiple goroutines call it.
type NaiveGenerator struct {
	mu     sync.Mutex
	used   map[uint64]struct{}
	rng    *rand.Rand
	closed bool
}

func NewNaiveGenerator() *NaiveGenerator {
	return &NaiveGenerator{
		used: make(map[uint64]struct{}),
		rng:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (g *NaiveGenerator) GetOneID() (uint64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return 0, errors.New("generator is closed")
	}

	for {
		id := g.rng.Uint64()
		if _, exists := g.used[id]; exists {
			continue
		}
		g.used[id] = struct{}{}
		return id, nil
	}
}

func (g *NaiveGenerator) Close() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
}

// ============================================================
// 2. Abstract batch ID source.
// ============================================================
// The reported Pure Storage problem can be viewed as:
// getIds(n) -> n unique IDs
// and then optimizing getOneId() by consuming batches locally.
type IDSource interface {
	GetIDs(n int) ([]uint64, error)
}

// AtomicIDSource is a simple source where each batch contains a
// contiguous unique range. The atomic counter makes it safe for
// multiple callers and also makes the GetIDs call cheap.
type AtomicIDSource struct {
	next atomic.Uint64
}

func NewAtomicIDSource(start uint64) *AtomicIDSource {
	s := &AtomicIDSource{}
	s.next.Store(start)
	return s
}

func (s *AtomicIDSource) GetIDs(n int) ([]uint64, error) {
	if n <= 0 {
		return nil, errors.New("batch size must be positive")
	}

	// Reserve a contiguous range atomically.
	start := s.next.Add(uint64(n)) - uint64(n)
	ids := make([]uint64, n)
	for i := range ids {
		ids[i] = start + uint64(i)
	}
	return ids, nil
}

// ============================================================
// 3. Single-buffer generator.
// ============================================================
// Basic design:
//
//	GetOneID -> consume local buffer
//	buffer empty -> fetch next batch
//
// The implementation deliberately does NOT hold mu while calling
// the potentially expensive source.GetIDs(). It uses a refill
// condition so that only one goroutine refills and the others wait.
type SingleBufferGenerator struct {
	mu        sync.Mutex
	cond      *sync.Cond
	source    IDSource
	batch     int
	buffer    []uint64
	closed    bool
	refilling bool
}

func NewSingleBufferGenerator(source IDSource, batchSize int) *SingleBufferGenerator {
	if batchSize <= 0 {
		panic("batch size must be positive")
	}
	g := &SingleBufferGenerator{
		source: source,
		batch:  batchSize,
	}
	g.cond = sync.NewCond(&g.mu)
	return g
}

func (g *SingleBufferGenerator) refill() error {
	ids, err := g.source.GetIDs(g.batch)

	g.mu.Lock()
	defer g.mu.Unlock()

	if err != nil {
		g.refilling = false
		g.cond.Broadcast()
		return err
	}
	if g.closed {
		g.refilling = false
		g.cond.Broadcast()
		return errors.New("generator is closed")
	}

	g.buffer = ids
	g.refilling = false
	g.cond.Broadcast()
	return nil
}

func (g *SingleBufferGenerator) GetOneID() (uint64, error) {
	for {
		g.mu.Lock()

		if g.closed {
			g.mu.Unlock()
			return 0, errors.New("generator is closed")
		}

		if len(g.buffer) > 0 {
			id := g.buffer[len(g.buffer)-1]
			g.buffer = g.buffer[:len(g.buffer)-1]
			g.mu.Unlock()
			return id, nil
		}

		if !g.refilling {
			g.refilling = true
			g.mu.Unlock()
			return g.refillAndRetry()
		}

		g.cond.Wait()
		g.mu.Unlock()
	}
}

func (g *SingleBufferGenerator) refillAndRetry() (uint64, error) {
	if err := g.refill(); err != nil {
		return 0, err
	}
	return g.GetOneID()
}

func (g *SingleBufferGenerator) Close() {
	g.mu.Lock()
	if !g.closed {
		g.closed = true
		g.buffer = nil
		g.cond.Broadcast()
	}
	g.mu.Unlock()
}

// ============================================================
// 4. Double-buffer generator.
// ============================================================
// Two batches are maintained. One is active for readers while the
// inactive side can be filled. This sample is intentionally kept
// interview-friendly rather than over-optimized.
type DoubleBufferGenerator struct {
	mu       sync.Mutex
	cond     *sync.Cond
	source   IDSource
	batch    int
	active   []uint64
	standby  []uint64
	filling  bool
	closed   bool
	prefetch bool
}

func NewDoubleBufferGenerator(source IDSource, batchSize int) *DoubleBufferGenerator {
	if batchSize <= 0 {
		panic("batch size must be positive")
	}
	g := &DoubleBufferGenerator{
		source:   source,
		batch:    batchSize,
		prefetch: true,
	}
	g.cond = sync.NewCond(&g.mu)
	return g
}

func (g *DoubleBufferGenerator) fillStandbyAsync() {
	ids, err := g.source.GetIDs(g.batch)

	g.mu.Lock()
	defer g.mu.Unlock()

	g.filling = false
	if g.closed {
		g.cond.Broadcast()
		return
	}
	if err == nil {
		g.standby = ids
	}
	g.cond.Broadcast()
}

func (g *DoubleBufferGenerator) swapLocked() {
	g.active, g.standby = g.standby, g.active
}

func (g *DoubleBufferGenerator) GetOneID() (uint64, error) {
	for {
		g.mu.Lock()

		if g.closed {
			g.mu.Unlock()
			return 0, errors.New("generator is closed")
		}

		if len(g.active) > 0 {
			id := g.active[len(g.active)-1]
			g.active = g.active[:len(g.active)-1]

			// Once the active buffer becomes small, start filling the
			// standby buffer so the next swap can happen quickly.
			if g.prefetch && len(g.active) <= g.batch/4 && len(g.standby) == 0 && !g.filling {
				g.filling = true
				go g.fillStandbyAsync()
			}

			g.mu.Unlock()
			return id, nil
		}

		// Active is empty. If standby is ready, swap immediately.
		if len(g.standby) > 0 {
			g.swapLocked()
			g.mu.Unlock()
			continue
		}

		// Nothing is ready. Start a fill if needed and wait.
		if !g.filling {
			g.filling = true
			go g.fillStandbyAsync()
		}

		g.cond.Wait()
		g.mu.Unlock()
	}
}

func (g *DoubleBufferGenerator) Close() {
	g.mu.Lock()
	g.closed = true
	g.active = nil
	g.standby = nil
	g.cond.Broadcast()
	g.mu.Unlock()
}

// ============================================================
// 5. Generic bounded batch producer/consumer.
// ============================================================
// This demonstrates the "background refill + bounded queue +
// backpressure + shutdown" version of the problem.
type WorkerGenerator struct {
	source IDSource
	batch  int
	queue  chan []uint64

	mu     sync.Mutex
	closed bool
	cancel context.CancelFunc
	wg     sync.WaitGroup

	current []uint64
}

func NewWorkerGenerator(source IDSource, batchSize, queueCapacity int) *WorkerGenerator {
	if batchSize <= 0 || queueCapacity <= 0 {
		panic("batch size and queue capacity must be positive")
	}

	ctx, cancel := context.WithCancel(context.Background())
	g := &WorkerGenerator{
		source: source,
		batch:  batchSize,
		queue:  make(chan []uint64, queueCapacity),
		cancel: cancel,
	}

	g.wg.Add(1)
	go g.producer(ctx)
	return g
}

func (g *WorkerGenerator) producer(ctx context.Context) {
	defer g.wg.Done()
	for {
		ids, err := g.source.GetIDs(g.batch)
		if err != nil {
			return
		}

		select {
		case g.queue <- ids:
			// Successful batch enqueue.
		case <-ctx.Done():
			return
		}
	}
}

func (g *WorkerGenerator) GetOneID() (uint64, error) {
	for {
		g.mu.Lock()
		if g.closed {
			g.mu.Unlock()
			return 0, errors.New("generator is closed")
		}
		if len(g.current) > 0 {
			id := g.current[len(g.current)-1]
			g.current = g.current[:len(g.current)-1]
			g.mu.Unlock()
			return id, nil
		}
		g.mu.Unlock()

		select {
		case batch := <-g.queue:
			g.mu.Lock()
			if g.closed {
				g.mu.Unlock()
				return 0, errors.New("generator is closed")
			}
			g.current = batch
			g.mu.Unlock()
		case <-time.After(50 * time.Millisecond):
			// Re-check closed state and then continue waiting.
		}
	}
}

func (g *WorkerGenerator) Close() {
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return
	}
	g.closed = true
	g.current = nil
	g.mu.Unlock()

	// Cancel producer and wait for it to exit.
	g.cancel()
	g.wg.Wait()
}

// ============================================================
// 6. Atomic counter generator.
// ============================================================
// If requirements allow monotonically increasing IDs, this is the
// simplest and fastest local implementation.
type AtomicCounterGenerator struct {
	next atomic.Uint64
}

func NewAtomicCounterGenerator(start uint64) *AtomicCounterGenerator {
	g := &AtomicCounterGenerator{}
	g.next.Store(start)
	return g
}

func (g *AtomicCounterGenerator) GetOneID() uint64 {
	return g.next.Add(1) - 1
}

// ============================================================
// 7. Range allocator + local range generator.
// ============================================================
// This simulates a distributed setup:
//
//	central allocator reserves [start, end)
//	each machine consumes its reserved range locally.
type IDRange struct {
	Start uint64
	End   uint64 // exclusive
}

type GlobalRangeAllocator struct {
	next atomic.Uint64
}

func NewGlobalRangeAllocator(start uint64) *GlobalRangeAllocator {
	a := &GlobalRangeAllocator{}
	a.next.Store(start)
	return a
}

func (a *GlobalRangeAllocator) Allocate(size uint64) IDRange {
	start := a.next.Add(size) - size
	return IDRange{Start: start, End: start + size}
}

type LocalRangeGenerator struct {
	mu     sync.Mutex
	alloc  *GlobalRangeAllocator
	chunk  uint64
	next   uint64
	end    uint64
	closed bool
}

func NewLocalRangeGenerator(alloc *GlobalRangeAllocator, chunk uint64) *LocalRangeGenerator {
	if chunk == 0 {
		panic("chunk must be positive")
	}
	return &LocalRangeGenerator{alloc: alloc, chunk: chunk}
}

func (g *LocalRangeGenerator) GetOneID() (uint64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return 0, errors.New("generator is closed")
	}

	if g.next == g.end {
		r := g.alloc.Allocate(g.chunk)
		g.next = r.Start
		g.end = r.End
	}

	id := g.next
	g.next++
	return id, nil
}

func (g *LocalRangeGenerator) Close() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
}

// ============================================================
// Demonstrations
// ============================================================
func demoNaive() {
	g := NewNaiveGenerator()
	defer g.Close()

	fmt.Println("Naive:")
	for i := 0; i < 3; i++ {
		id, _ := g.GetOneID()
		fmt.Println(" ", id)
	}
}

func demoSingleBuffer() {
	source := NewAtomicIDSource(1000)
	g := NewSingleBufferGenerator(source, 8)
	defer g.Close()

	fmt.Println("\nSingle buffer:")
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				id, err := g.GetOneID()
				if err != nil {
					fmt.Println(" error:", err)
					return
				}
				fmt.Printf(" worker=%d id=%d\n", worker, id)
			}
		}(worker)
	}
	wg.Wait()
}

func demoDoubleBuffer() {
	source := NewAtomicIDSource(10000)
	g := NewDoubleBufferGenerator(source, 16)
	defer g.Close()

	fmt.Println("\nDouble buffer:")
	for i := 0; i < 20; i++ {
		id, err := g.GetOneID()
		if err != nil {
			fmt.Println(" error:", err)
			return
		}
		fmt.Println(" ", id)
	}
}

func demoWorker() {
	source := NewAtomicIDSource(20000)
	g := NewWorkerGenerator(source, 16, 4)

	fmt.Println("\nBackground worker / bounded queue:")
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				id, err := g.GetOneID()
				if err != nil {
					fmt.Println(" error:", err)
					return
				}
				if i == 0 {
					fmt.Printf(" worker=%d firstID=%d\n", worker, id)
				}
			}
		}(worker)
	}
	wg.Wait()
	g.Close()
}

func demoAtomicCounter() {
	g := NewAtomicCounterGenerator(50000)

	fmt.Println("\nAtomic counter:")
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 3; i++ {
				fmt.Println(" ", g.GetOneID())
			}
		}()
	}
	wg.Wait()
}

func demoDistributedRanges() {
	allocator := NewGlobalRangeAllocator(1_000_000)
	machineA := NewLocalRangeGenerator(allocator, 1000)
	machineB := NewLocalRangeGenerator(allocator, 1000)
	defer machineA.Close()
	defer machineB.Close()

	fmt.Println("\nDistributed range allocation:")
	for i := 0; i < 5; i++ {
		a, _ := machineA.GetOneID()
		b, _ := machineB.GetOneID()
		fmt.Printf(" machineA=%d machineB=%d\n", a, b)
	}
}

func main() {
	demoNaive()
	demoSingleBuffer()
	demoDoubleBuffer()
	demoWorker()
	demoAtomicCounter()
	demoDistributedRanges()
}
