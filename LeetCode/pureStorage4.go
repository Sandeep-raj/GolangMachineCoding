package main

import (
	"fmt"
	"sync"
	"time"
)

type cb func()
type PriorityLock struct {
	mu          *sync.Mutex
	writerCount int
	readerCount int
	cond        *sync.Cond
	idmap       map[int]int
}

func createLock() PriorityLock {
	m := &sync.Mutex{}
	return PriorityLock{
		mu:    m,
		cond:  sync.NewCond(m),
		idmap: make(map[int]int),
	}
}

var lock PriorityLock = createLock()

func (l *PriorityLock) RLock(id int) {
	l.mu.Lock()

	if l.idmap[id] > 0 {
		l.idmap[id]++
		l.readerCount++
		l.mu.Unlock()
		return
	}

	l.idmap[id]++
	for l.writerCount > 0 {
		l.cond.Wait()
	}
	l.readerCount++
	l.mu.Unlock()
}

func (l *PriorityLock) RUnlock(id int) {
	l.mu.Lock()

	l.idmap[id]--
	l.readerCount--

	if l.idmap[id] == 0 {
		delete(l.idmap, id)
	}

	if l.readerCount == 0 && l.writerCount > 0 {
		l.cond.Broadcast()
	}
	l.mu.Unlock()
}

func (l *PriorityLock) Lock() {
	l.mu.Lock()
	l.writerCount++

	for l.readerCount > 0 {
		l.cond.Wait()
	}
	l.mu.Unlock()
}

func (l *PriorityLock) Unlock() {
	l.mu.Lock()
	l.writerCount--
	l.cond.Broadcast()
	l.mu.Unlock()
}

func regCb(fn cb, id int) {
	lock.RLock(id)
	defer lock.RUnlock(id)
	fmt.Println("cb is executing")
	fn()
	fmt.Println("cb is completed")
}

func execEvents() {
	lock.Lock()
	defer lock.Unlock()
	fmt.Println("event is executing")
	time.Sleep(5 * time.Second)
	fmt.Println("event is completed")
}

func main() {
	wg := sync.WaitGroup{}

	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	regCb(func() {
	// 		time.Sleep(2 * time.Second)
	// 	}, 1)
	// }()
	time.Sleep(100 * time.Millisecond)
	wg.Add(1)
	go func() {
		defer wg.Done()
		execEvents()
	}()

	time.Sleep(1 * time.Second)
	wg.Add(1)
	go func() {
		defer wg.Done()
		execEvents()
	}()
	wg.Add(1)

	go func() {
		defer wg.Done()
		regCb(func() {
			time.Sleep(2 * time.Second)
		}, 2)
	}()

	time.Sleep(2 * time.Second)
	wg.Add(1)
	go func() {
		defer wg.Done()
		regCb(func() {
			time.Sleep(2 * time.Second)
		}, 3)
	}()

	wg.Wait()
	fmt.Println("Workflow Completed")
}
