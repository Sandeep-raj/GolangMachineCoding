package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
)

/*
FAN IN

Worker 1 ──┐
Worker 2 ──┼──> merged
Worker 3 ──┘
*/

func worker(ch chan string, wg *sync.WaitGroup, id int, random int) {
	val := rand.Intn(random)
	defer wg.Done()

	for i := range val {
		ch <- fmt.Sprintf("task-%d-worker-%d", i, id)
	}
	log.Print("worker ", id, " completed the task")
}

func main() {
	ch := make(chan string)
	consCh := make(chan struct{})
	wg := sync.WaitGroup{}

	// run all the consumers
	go func() {
		for res := range ch {
			log.Print(res)
		}
		consCh <- struct{}{}
	}()

	// run all the workers
	for i := range 3 {
		wg.Add(1)
		go worker(ch, &wg, i+1, 100)
	}

	wg.Wait()
	close(ch)

	<-consCh
	close(consCh)
	log.Print("All task completed")
}
