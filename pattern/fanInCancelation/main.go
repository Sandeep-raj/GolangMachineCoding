package main

import (
	"context"
	"log"
	"sync"
)

func merge(ctx context.Context, chs ...chan int) chan int {
	cout := make(chan int)
	wg := sync.WaitGroup{}

	wg.Add(len(chs))

	go func() {
		for _, ch := range chs {
			go func(ch chan int) {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					case v, ok := <-ch:
						if !ok {
							return
						}

						select {
						case <-ctx.Done():
							return
						case cout <- v:
						}
					}
				}
			}(ch)
		}
	}()

	go func() {
		wg.Wait()
		close(cout)
	}()

	return cout
}

func producer(i int, ctx context.Context) chan int {
	ch := make(chan int)

	go func() {
		defer close(ch)
		for v := range i {
			select {
			case <-ctx.Done():
				return
			case ch <- v:
			}
		}
	}()

	return ch
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	ch1 := producer(10, ctx)
	ch2 := producer(19, ctx)

	resch := merge(ctx, ch1, ch2)

	for v := range resch {
		log.Print(v)

		if v >= 15 {
			cancel()
		}
	}
	log.Print("completed")
}
