package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

/*
FAN IN

Worker 1 ──┐  ch    |-- Consumer 1
Worker 2 ──┼──> ----|
Worker 3 ──┘        |-- Consumer 2
*/

func worker(ch chan string, wg *sync.WaitGroup, id int, random int) {
	val := rand.Intn(random)
	defer wg.Done()

	for i := range val {
		time.Sleep(time.Duration(val%3) * time.Second)
		ch <- fmt.Sprintf("task-%d-worker-%d", i, id)
	}
	log.Print("worker ", id, " completed the task")
}

func consumer(ch chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	for val := range ch {
		log.Print(val)
	}
}

func main() {
	ch := make(chan string)
	cwg := sync.WaitGroup{}
	pwg := sync.WaitGroup{}

	// run all the consumers
	for range 2 {
		cwg.Add(1)
		go consumer(ch, &cwg)
	}

	// run all the workers
	for i := range 3 {
		pwg.Add(1)
		go worker(ch, &pwg, i+1, 100)
	}

	pwg.Wait()
	close(ch)

	cwg.Wait()
	log.Print("All task completed")
}
