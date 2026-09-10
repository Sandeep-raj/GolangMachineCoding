package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type cb func()

type MultiThread struct {
	fired    atomic.Uint32
	mu       *sync.RWMutex
	cbs      []cb
	executor func(fn cb)
}

func createThread() *MultiThread {
	m := sync.RWMutex{}
	return &MultiThread{
		mu:  &m,
		cbs: make([]cb, 0),
		executor: func(fn cb) {
			fn()
		},
	}
}

func (t *MultiThread) regcb(fn cb) {
	if t.fired.Load() == 1 {
		t.executor(fn)
		return
	}

	execfn := false

	t.mu.Lock()
	if t.fired.Load() == 1 {
		execfn = true
	} else {
		t.cbs = append(t.cbs, fn)
	}
	t.mu.Unlock()

	if execfn {
		t.executor(fn)
	}
}

func (t *MultiThread) evtfire() {
	if t.fired.Load() == 1 {
		return
	}

	t.mu.Lock()
	t.fired.Store(1)

	waitingcb := t.cbs
	t.cbs = make([]cb, 0)
	t.mu.Unlock()

	for _, c := range waitingcb {
		t.executor(c)
	}
}

func main() {
	t := createThread()
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		t.regcb(func() {
			defer wg.Done()
			fmt.Println("log 1")
		})
	}()

	wg.Add(1)
	go func() {
		t.regcb(func() {
			defer wg.Done()
			fmt.Println("log 2")
		})
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		t.evtfire()
	}()

	wg.Wait()

}
