package main

import (
	"log"
	"sync"
)

/*
TEE
             ┌── out1
input ───────┤
             └── out2
*/

func tee(in chan int) (chan int, chan int) {
	cout1 := make(chan int)
	cout2 := make(chan int)

	go func() {
		defer close(cout1)
		defer close(cout2)

		for v := range in {
			cout1 <- v
			cout2 <- v
		}
	}()

	return cout1, cout2
}

func main() {
	inChan := make(chan int)
	wg := sync.WaitGroup{}
	wg.Add(2)

	a, b := tee(inChan)

	// producer
	go func() {
		defer close(inChan)
		for i := range 10 {
			inChan <- i
		}
	}()

	// consumers
	go func() {
		defer wg.Done()
		for v := range a {
			log.Print(v)
		}
	}()

	go func() {
		defer wg.Done()
		for v := range b {
			log.Print(v)
		}
	}()

	wg.Wait()
}
