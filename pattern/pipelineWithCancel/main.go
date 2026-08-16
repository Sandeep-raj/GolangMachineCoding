package main

import (
	"context"
	"log"
)

func gen(ctx context.Context, in ...int) chan int {
	cout := make(chan int)

	go func(in []int) {
		defer close(cout)

		for _, v := range in {
			select {
			case cout <- v:
			case <-ctx.Done():
				return
			}
		}
	}(in)

	return cout
}

func square(ctx context.Context, in chan int) chan int {
	cout := make(chan int)

	go func(in chan int) {
		defer close(cout)

		for v := range in {
			select {
			case cout <- v * v:
			case <-ctx.Done():
				return
			}
		}
	}(in)

	return cout
}

func double(ctx context.Context, in chan int) chan int {
	cout := make(chan int)

	go func(in chan int) {
		defer close(cout)

		for v := range in {
			select {
			case cout <- 2 * v:
			case <-ctx.Done():
				return
			}
		}
	}(in)

	return cout
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	genc := gen(ctx, 1, 2, 3, 4)
	sqc := square(ctx, genc)

	res := <-sqc
	cancel()

	log.Print("recv result ", res)
}
