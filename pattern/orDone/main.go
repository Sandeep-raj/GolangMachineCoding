package main

import "log"

func orDone(done chan struct{}, in chan int) chan int {
	cout := make(chan int)

	go func() {
		defer close(cout)

		for {
			select {
			case <-done:
				return
			case v, ok := <-in:
				if !ok {
					return
				}

				select {
				case <-done:
					return
				default:
					cout <- 2 * v
				}
			}
		}
	}()

	return cout
}

func main() {
	in := make(chan int)
	done := make(chan struct{})

	// produce in
	go func() {
		defer close(in)
		for i := range 10 {
			in <- i
		}
	}()

	// result
	ch := orDone(done, in)
	for v := range ch {
		if v > 15 {
			close(done)
		}

		log.Print(v)
	}
}
