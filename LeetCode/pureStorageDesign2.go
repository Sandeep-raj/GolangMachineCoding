package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	epoch  int64 = time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	seqBit int   = 12
	macbit int   = 5
	dcbit  int   = 5
	maxseq int   = (1 << seqBit) - 1
)

type AtomicIDGenerator struct {
	dc    int
	mac   int
	state atomic.Int64
}

func createIDGenerator(d, m int) *AtomicIDGenerator {
	return &AtomicIDGenerator{
		state: atomic.Int64{},
		dc:    d,
		mac:   m,
	}
}

func (ug *AtomicIDGenerator) NextID() int64 {
	for {
		now := int64(time.Now().UnixMilli() - epoch)

		old := ug.state.Load()
		lasttime := old >> int64(seqBit)
		lastseq := old & int64(maxseq)

		if now < lasttime {
			time.Sleep(1 * time.Microsecond)
			continue
		}

		if now == lasttime {
			if lastseq >= int64(maxseq) {
				time.Sleep(1 * time.Microsecond)
				continue
			}
			lastseq++
		} else {
			lastseq = 0
		}

		newstate := (now << int64(seqBit)) | lastseq
		if ug.state.CompareAndSwap(old, newstate) {
			// fmt.Println("time ", now, " seq ", lastseq)
			return ((now)<<(dcbit+macbit+seqBit) |
				int64(ug.dc)<<(seqBit+macbit) |
				int64(ug.mac)<<seqBit |
				lastseq)
		}
	}
}

func main() {
	gen := createIDGenerator(1, 1)

	var wg sync.WaitGroup

	for i := 0; i < 4028; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			id := gen.NextID()
			fmt.Println(id)
		}()
	}

	wg.Wait()

	fmt.Println("All goroutines completed")
}
