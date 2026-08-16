package main

import "fmt"

func main() {
	ch1 := make(chan int)
	ch2 := make(chan int)

	go func() {
		ch1 <- 10
		close(ch1)
	}()

	go func() {
		ch2 <- 20
		close(ch2)
	}()

	for ch1 != nil || ch2 != nil {
		select {
		case v, ok := <-ch1:
			if !ok {
				// Disable this select case.
				ch1 = nil
				continue
			}

			fmt.Println("ch1:", v)

		case v, ok := <-ch2:
			if !ok {
				// Disable this select case.
				ch2 = nil
				continue
			}

			fmt.Println("ch2:", v)
		}
	}
}
