package main

import "log"

func gen(nums ...int) chan int {
	cout := make(chan int)

	go func(nums []int) {
		defer close(cout)

		for _, v := range nums {
			cout <- v
		}
	}(nums)

	return cout
}

func square(inchan chan int) chan int {
	cout := make(chan int)

	go func(in chan int) {
		defer close(cout)

		for v := range in {
			cout <- v * v
		}
	}(inchan)

	return cout
}

func double(inchan chan int) chan int {
	cout := make(chan int)

	go func(in chan int) {
		defer close(cout)

		for v := range in {
			cout <- 2 * v
		}
	}(inchan)

	return cout
}

func main() {
	genc := gen(1, 2, 3, 4)
	sqc := square(genc)
	dc := double(sqc)

	for v := range dc {
		log.Print(v)
	}
}
