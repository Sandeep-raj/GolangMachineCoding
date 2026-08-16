package main

import "log"

/*
chan of channels

outer
  │
  ├── ch1 → 1 2 3
  ├── ch2 → 4 5
  └── ch3 → 6 7 8

       ↓

output → 1 2 3 4 5 6 7 8
*/

func producer(nums ...int) chan int {
	cout := make(chan int)

	go func(nums []int) {
		defer close(cout)
		for _, v := range nums {
			cout <- v
		}
	}(nums)

	return cout
}

func bridge(in chan chan int) chan int {
	cout := make(chan int)

	go func(in chan chan int, cout chan int) {
		defer close(cout)
		for c := range in {
			for v := range c {
				cout <- v
			}
		}
	}(in, cout)

	return cout
}

func main() {
	ch1 := producer(1, 2, 3)
	ch2 := producer(4, 5)
	ch3 := producer(6, 7, 8)

	ch := make(chan chan int)
	c := bridge(ch)

	go func() {
		defer close(ch)
		ch <- ch1
		ch <- ch2
		ch <- ch3
	}()

	for v := range c {
		log.Print(v)
	}
}
