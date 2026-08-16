package main

import (
	"log"
	"sync"
)

/*
FAN OUT
                    ┌── Worker 1
                    │
jobs ────────────────┼── Worker 2
                    │
                    └── Worker 3
*/

func worker(in chan int, id int, wg *sync.WaitGroup) {
	for v := range in {
		log.Printf("%d job processed by %d worker", v, id)
	}

	wg.Done()
	log.Print("worker closed")
}

func main() {
	wg := sync.WaitGroup{}
	ch := make(chan int)

	for i := range 3 {
		wg.Add(1)

		go worker(ch, i, &wg)
	}

	// start the producer
	go func() {
		for i := range 15 {
			ch <- i
		}
		close(ch)
	}()

	wg.Wait()

	log.Print("all tasks completed")
}
