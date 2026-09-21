package main

import (
	"fmt"
	"sync"
	"time"
	"sync/atomic"
)

var (
	epoch int64 = time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

	dcbit  int8 = 5
	macbit int8 = 5
	seqbit int8 = 12

	maxseq int16 = (1 << seqbit) - 1

	globalLock sync.Mutex = sync.Mutex{}
	blkSize int64 = 20000
	currSeq int64 = 1000000
)

// Block Uniq Genrator

type UIDGenBlock struct {
	mu *sync.Mutex
	seqno atomic.Int64

	endSeq int64
}

func allocateBlock(gen *UIDGenBlock) {
	globalLock.Lock()
	defer globalLock.Unlock()

	seq := currSeq
	currSeq += blkSize

	gen.seqno.Store(seq)
	gen.endSeq = seq + blkSize
}

func createUIDGenBlock() *UIDGenBlock {
	gen := UIDGenBlock{
		mu: &sync.Mutex{},
		seqno: atomic.Int64{},
	}

	allocateBlock(&gen)

	return &gen;
}

func (ugen *UIDGenBlock) NextID() int64 {
	ugen.mu.Lock()

	if(ugen.seqno.Load() == ugen.endSeq) {
		allocateBlock(ugen)
	}
	ugen.mu.Unlock()

	return ugen.seqno.Add(1)
}

///////////////////////////////////////////////////////////////

// Atomic Uniq Genrator

type UIDGenAtomic struct {
	dc int64
	macid int64
	state atomic.Int64
}

func createUIDGenAtomic(d, m int64) *UIDGenAtomic {
	gen := UIDGenAtomic{
		dc : d,
		macid: m,
		state: atomic.Int64{},
	}

	return &gen
}

func (ugen *UIDGenAtomic) NextID() int64 {
	for {
		currState := ugen.state.Load()
		lastsync := (currState >> seqbit)
		seqno := (currState & int64(maxseq))

		now := time.Now().UnixMilli() - epoch

		if(now < lastsync) {
			fmt.Println("clock is skewed")
			continue
		}

		if(now == lastsync) {
			seqno++;

			if(seqno & int64(maxseq) == 0) {
				time.Sleep(1 * time.Microsecond)
				continue
			}
		} else {
			seqno = 0
		}

		if ugen.state.CompareAndSwap( currState , (now << seqbit) | seqno ) {
			return (now << (seqbit + macbit + dcbit)) | (ugen.dc << (seqbit + macbit)) | (ugen.macid << seqbit) | seqno;
		}
		time.Sleep(1 * time.Microsecond)
	}
}

///////////////////////////////////////////////////////////////

// Lock Uniq Genrator

type UIDGenLock struct {
	mu       *sync.Mutex
	dc       int8
	macid    int8
	seqno    int16
	lastsync int64
}

func createUIDLock(d, m int8) *UIDGenLock {
	uidgen := UIDGenLock{
		mu:    &sync.Mutex{},
		dc:    d,
		macid: m,
	}

	return &uidgen
}

func (ugen *UIDGenLock) NextID() int64 {
	ugen.mu.Lock()
	defer ugen.mu.Unlock()

	now := time.Now().UnixMilli()
	if now < ugen.lastsync {
		fmt.Println("time is skewed...")
		return -1
	}

	if now == ugen.lastsync {
		ugen.seqno++

		if ugen.seqno&maxseq == 0 {
			now = ugen.waitNextMS(now)
			ugen.seqno = 0
		}
	} else {
		ugen.seqno = 0
	}

	ugen.lastsync = now

	return ((ugen.lastsync - epoch) << (seqbit + macbit + dcbit)) | (int64(ugen.dc) << (seqbit + macbit)) | (int64(ugen.macid) << (seqbit)) | int64(ugen.seqno)
}

func (ugen *UIDGenLock) waitNextMS(currtime int64) int64 {
	now := time.Now().UnixMilli()

	for now <= currtime {
		time.Sleep(1 * time.Microsecond)
		now = time.Now().UnixMilli()
	}

	return now
}

///////////////////////////////////////////////////////////////

func main() {
	// fmt.Printf("Hello, World!")

	gen := createUIDLock(1, 1)
	id := gen.NextID()

	fmt.Println(id)

	gen2 := createUIDGenAtomic(1, 1)
	id2 := gen2.NextID()

	fmt.Println(id2)

	gen3 := createUIDGenBlock()

	var wg sync.WaitGroup

	for i := 0; i < 4028; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			id := gen3.NextID()
			fmt.Println(id)
		}()
	}

	wg.Wait()

	fmt.Println("All goroutines completed")
}
