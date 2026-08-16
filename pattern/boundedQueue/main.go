package main

import "fmt"

func submit(jobs chan<- int, job int) bool {
	select {
	case jobs <- job:
		return true

	default:
		return false
	}
}

func main() {
	jobs := make(chan int, 2)

	fmt.Println(submit(jobs, 1)) // true
	fmt.Println(submit(jobs, 2)) // true
	fmt.Println(submit(jobs, 3)) // false

	fmt.Println("queue:", len(jobs))
}
