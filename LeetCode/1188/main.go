package main

import (
	"log"
	"sync"
	"time"
)

/*
LeetCode 1188 — Design Bounded Blocking Queue

Here is the problem statement:

Implement a thread-safe bounded blocking queue that has the following operations:

enqueue(element): Adds an element to the queue. If the queue is full, the calling thread should block until the queue is no longer full.
dequeue(): Returns the element at the front of the queue and removes it. If the queue is empty, the calling thread should block until the queue is no longer empty.
size(): Returns the current number of elements in the queue.

The queue should be initialized with a fixed capacity.

Example

Given:

capacity = 2

Operations:

enqueue(1)
enqueue(2)
enqueue(3)

The third enqueue(3) must block because the queue is already full.

After another thread executes:

dequeue()

the blocked enqueue(3) can proceed.

Constraints
1 <= capacity <= 1000
1 <= element <= 1000
Multiple threads may call enqueue and dequeue concurrently.
enqueue must block when the queue is full.
dequeue must block when the queue is empty.

This is a classic sync.Cond problem in Go because you have two conditions:

notFull  → enqueue can proceed
notEmpty → dequeue can proceed

A natural implementation uses:

sync.Mutex
sync.Cond

with a queue underneath.
*/

type Queue struct {
	m    *sync.Mutex
	q    []int
	n    int
	cond sync.Cond
}

func NewQueue(x int) *Queue {
	lock := sync.Mutex{}
	return &Queue{
		n:    x,
		q:    make([]int, 0, x),
		m:    &lock,
		cond: *sync.NewCond(&lock),
	}
}

func (q *Queue) Enqueue(x int) {
	q.m.Lock()

	for len(q.q) == q.n {
		q.cond.Wait()
	}

	q.q = append(q.q, x)
	q.cond.Broadcast()

	q.m.Unlock()
}

func (q *Queue) Dequeue() int {
	q.m.Lock()

	for len(q.q) == 0 {
		q.cond.Wait()
	}

	res := q.q[0]
	q.q = q.q[1:]
	q.cond.Broadcast()

	q.m.Unlock()

	return res
}

func (q *Queue) Size() int {
	q.m.Lock()
	defer q.m.Unlock()

	return len(q.q)
}

func main() {
	q := NewQueue(5)
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		for i := range 10 {
			q.Enqueue(i)
		}
	}()

	go func() {
		for {
			val := q.Dequeue()
			log.Print(val)
		}
	}()

	wg.Wait()
	time.Sleep(1 * time.Second)
}
