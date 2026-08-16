package main

import (
	"fmt"
	"time"
)

func worker(done <-chan struct{}) {
	for {
		select {
		case <-done:
			fmt.Println("worker stopping")
			return

		default:
			fmt.Println("working...")
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func main() {
	done := make(chan struct{})

	go worker(done)

	time.Sleep(time.Second)

	close(done)

	time.Sleep(100 * time.Millisecond)
}
