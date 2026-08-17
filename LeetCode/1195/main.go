package main

import (
	"log"
	"sync"
	"time"
)

/*
1195. Fizz Buzz Multithreaded

You have the four functions:

printFizz that prints the word "fizz" to the console,
printBuzz that prints the word "buzz" to the console,
printFizzBuzz that prints the word "fizzbuzz" to the console, and
printNumber that prints a given integer to the console.
You are given an instance of the class FizzBuzz that has four functions: fizz, buzz, fizzbuzz and number. The same instance of FizzBuzz will be passed to four different threads:

Thread A: calls fizz() that should output the word "fizz".
Thread B: calls buzz() that should output the word "buzz".
Thread C: calls fizzbuzz() that should output the word "fizzbuzz".
Thread D: calls number() that should only output the integers.
Modify the given class to output the series [1, 2, "fizz", 4, "buzz", ...] where the ith token (1-indexed) of the series is:

"fizzbuzz" if i is divisible by 3 and 5,
"fizz" if i is divisible by 3 and not 5,
"buzz" if i is divisible by 5 and not 3, or
i if i is not divisible by 3 or 5.
Implement the FizzBuzz class:

FizzBuzz(int n) Initializes the object with the number n that represents the length of the sequence that should be printed.
void fizz(printFizz) Calls printFizz to output "fizz".
void buzz(printBuzz) Calls printBuzz to output "buzz".
void fizzbuzz(printFizzBuzz) Calls printFizzBuzz to output "fizzbuzz".
void number(printNumber) Calls printnumber to output the numbers.


Example 1:

Input: n = 15
Output: [1,2,"fizz",4,"buzz","fizz",7,8,"fizz","buzz",11,"fizz",13,14,"fizzbuzz"]
Example 2:

Input: n = 5
Output: [1,2,"fizz",4,"buzz"]
*/

type PrintFizzBuzz struct {
	m    *sync.Mutex
	cond sync.Cond
	n    int
	i    int
}

func NewFizzBuzz(x int) *PrintFizzBuzz {
	lock := sync.Mutex{}
	return &PrintFizzBuzz{
		m:    &lock,
		cond: *sync.NewCond(&lock),
		n:    x,
		i:    1,
	}
}

func (pfb *PrintFizzBuzz) printFizz() {
	pfb.m.Lock()

	for pfb.i <= pfb.n {
		for pfb.i%3 != 0 || pfb.i%5 == 0 {
			pfb.cond.Wait()
		}

		log.Print("fizz", pfb.i)
		pfb.i++

		if pfb.i > pfb.n {
			return
		}

		pfb.cond.Broadcast()
	}

	pfb.m.Unlock()
}

func (pfb *PrintFizzBuzz) printBuzz() {
	pfb.m.Lock()

	for pfb.i <= pfb.n {
		for pfb.i%3 == 0 || pfb.i%5 != 0 {
			pfb.cond.Wait()
		}

		log.Print("buzz", pfb.i)
		pfb.i++

		if pfb.i > pfb.n {
			return
		}

		pfb.cond.Broadcast()
	}

	pfb.m.Unlock()
}

func (pfb *PrintFizzBuzz) printFizzBuzz() {
	pfb.m.Lock()

	for pfb.i <= pfb.n {
		for pfb.i%3 != 0 || pfb.i%5 != 0 {
			pfb.cond.Wait()
		}

		log.Print("fizzbuzz")
		pfb.i++

		if pfb.i > pfb.n {
			return
		}

		pfb.cond.Broadcast()
	}

	pfb.m.Unlock()
}

func (pfb *PrintFizzBuzz) printNumber() {
	pfb.m.Lock()

	for pfb.i <= pfb.n {
		for pfb.i%3 == 0 || pfb.i%5 == 0 {
			pfb.cond.Wait()
		}

		log.Print(pfb.i)
		pfb.i++

		if pfb.i > pfb.n {
			return
		}

		pfb.cond.Broadcast()
	}

	pfb.m.Unlock()
}

func main() {
	pfb := NewFizzBuzz(10)

	go pfb.printFizz()
	go pfb.printBuzz()
	go pfb.printFizzBuzz()
	go pfb.printNumber()

	time.Sleep(2 * time.Second)
}
