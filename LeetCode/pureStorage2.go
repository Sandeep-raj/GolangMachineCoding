package main

import (
	"fmt"
	"sync"
	"time"
)

type cb func()
type PriorityLock struct {
	mu            *sync.Mutex
	writerWaiting bool
	readerCount   int
	cond          *sync.Cond
}

func createLock() PriorityLock {
	m := &sync.Mutex{}
	return PriorityLock{
		mu:   m,
		cond: sync.NewCond(m),
	}
}

var lock PriorityLock = createLock()

func (l *PriorityLock) RLock() {
	l.mu.Lock()

	for l.writerWaiting {
		l.cond.Wait()
	}
	l.readerCount++
	l.mu.Unlock()
}

func (l *PriorityLock) RUnlock() {
	l.mu.Lock()

	l.readerCount--
	if l.readerCount == 0 && l.writerWaiting {
		l.cond.Broadcast()
	}
	l.mu.Unlock()
}

func (l *PriorityLock) Lock() {
	l.mu.Lock()
	l.writerWaiting = true

	for l.readerCount > 0 {
		l.cond.Wait()
	}
}

func (l *PriorityLock) Unlock() {
	l.writerWaiting = false
	l.cond.Broadcast()
	l.mu.Unlock()
}

func regCb(fn cb) {
	lock.RLock()
	defer lock.RUnlock()
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

	wg.Add(1)
	go func() {
		defer wg.Done()
		regCb(func() {
			time.Sleep(2 * time.Second)
		})
	}()
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
		regCb(func() {
			time.Sleep(2 * time.Second)
		})
	}()

	time.Sleep(2 * time.Second)
	wg.Add(1)
	go func() {
		defer wg.Done()
		regCb(func() {
			time.Sleep(2 * time.Second)
		})
	}()

	wg.Wait()
	fmt.Println("Workflow Completed")
}
