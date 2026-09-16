package main

import (
	"fmt"
	"sync"
	"time"
)

type UniqueGenerator struct {
	lasttimems int64
	counter    int64
	dc         uint8
	machineid  uint8
	mu         *sync.Mutex
}

var (
	epoch int64 = time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

	seqBit int = 12
	macid  int = 5
	dc     int = 5

	seqBitMax int64 = (1 << 12) - 1
	timeshift int   = seqBit + macid + dc
	macshift  int   = seqBit
	dcshift   int   = seqBit + macid
)

func createGenerator(d, m uint8) *UniqueGenerator {
	return &UniqueGenerator{
		lasttimems: -1,
		counter:    0,
		dc:         d,
		machineid:  m,
		mu:         &sync.Mutex{},
	}
}

func (uq *UniqueGenerator) nextId() int64 {
	uq.mu.Lock()
	defer uq.mu.Unlock()

	now := time.Now().UnixMilli()

	if now < uq.lasttimems {
		fmt.Println("clock is backward")
		return -1
	}

	if now == uq.lasttimems {
		if uq.counter == seqBitMax {
			// counter went higher then the maxcount
			now = uq.waitNextMS(now)
		}
	}

	if now == uq.lasttimems {
		uq.counter++
	} else {
		uq.counter = 0
	}

	uq.lasttimems = now

	return ((int64(now-epoch) << timeshift) |
		(int64(uq.dc) << dcshift) |
		(int64(uq.machineid) << macshift) |
		int64(uq.counter))
}

func (uq *UniqueGenerator) waitNextMS(lastmilis int64) int64 {
	now := time.Now().UnixMilli()

	for lastmilis <= now {
		time.Sleep(1 * time.Microsecond)
		now = time.Now().UnixMilli()
	}
	return now
}

func main() {
	gen := createGenerator(1, 1)

	var wg sync.WaitGroup

	for i := 0; i < 123; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			id := gen.nextId()
			fmt.Println(id)
		}()
	}

	wg.Wait()

	fmt.Println("All goroutines completed")
}
