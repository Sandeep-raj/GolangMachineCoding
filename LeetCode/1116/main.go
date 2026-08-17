package main

import (
	"log"
	"sync"
)

/*
1116. Print Zero Even Odd

You have a function printNumber that can be called with an integer parameter and prints it to the console.

For example, calling printNumber(7) prints 7 to the console.
You are given an instance of the class ZeroEvenOdd that has three functions: zero, even, and odd. The same instance of ZeroEvenOdd will be passed to three different threads:

Thread A: calls zero() that should only output 0's.
Thread B: calls even() that should only output even numbers.
Thread C: calls odd() that should only output odd numbers.
Modify the given class to output the series "010203040506..." where the length of the series must be 2n.

Implement the ZeroEvenOdd class:

ZeroEvenOdd(int n) Initializes the object with the number n that represents the numbers that should be printed.
void zero(printNumber) Calls printNumber to output one zero.
void even(printNumber) Calls printNumber to output one even number.
void odd(printNumber) Calls printNumber to output one odd number.


Example 1:

Input: n = 2
Output: "0102"
Explanation: There are three threads being fired asynchronously.
One of them calls zero(), the other calls even(), and the last one calls odd().
"0102" is the correct output.
Example 2:

Input: n = 5
Output: "0102030405"
*/

type ZeroEvenOdd struct {
	n  int
	zc chan struct{}
	ec chan struct{}
	oc chan struct{}
	wg *sync.WaitGroup
}

func CreateZEO(n int) ZeroEvenOdd {
	return ZeroEvenOdd{
		n:  n,
		zc: make(chan struct{}),
		ec: make(chan struct{}),
		oc: make(chan struct{}),
		wg: &sync.WaitGroup{},
	}
}

func (z *ZeroEvenOdd) zero() {
	odd := true
	for range z.zc {
		log.Print(0)

		if odd {
			z.oc <- struct{}{}
		} else {
			z.ec <- struct{}{}
		}
		odd = !odd
	}
}
func (z *ZeroEvenOdd) even() {
	defer close(z.ec)
	for i := 2; i <= z.n; i = i + 2 {
		<-z.ec
		z.wg.Done()
		log.Print(i)

		if i == z.n {
			close(z.zc)
			return
		}
		z.zc <- struct{}{}
	}
}

func (z *ZeroEvenOdd) odd() {
	defer close(z.oc)
	for i := 1; i <= z.n; i = i + 2 {
		<-z.oc
		z.wg.Done()
		log.Print(i)

		if i == z.n {
			close(z.zc)
			return
		}
		z.zc <- struct{}{}
	}
}

func main() {
	z := CreateZEO(4)
	z.wg.Add(4)

	go z.zero()
	go z.even()
	go z.odd()

	z.zc <- struct{}{}

	z.wg.Wait()
}
