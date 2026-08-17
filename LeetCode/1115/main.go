package main

import (
	"fmt"
	"sync"
)

/*
1115. Print FooBar Alternately

Suppose you are given the following code:

class FooBar {
  public void foo() {
    for (int i = 0; i < n; i++) {
      print("foo");
    }
  }

  public void bar() {
    for (int i = 0; i < n; i++) {
      print("bar");
    }
  }
}
The same instance of FooBar will be passed to two different threads:

thread A will call foo(), while
thread B will call bar().
Modify the given program to output "foobar" n times.



Example 1:

Input: n = 1
Output: "foobar"
Explanation: There are two threads being fired asynchronously. One of them calls foo(), while the other calls bar().
"foobar" is being output 1 time.
Example 2:

Input: n = 2
Output: "foobarfoobar"
Explanation: "foobar" is being output 2 times.
*/

type FooBar struct {
	n    int
	fooC chan struct{}
	barC chan struct{}
	wg   *sync.WaitGroup
}

func (f *FooBar) foo() {
	for i := 0; i < f.n; i++ {
		<-f.fooC
		fmt.Print("foo")
		f.barC <- struct{}{}
	}
}

func (f *FooBar) bar() {
	for i := 0; i < f.n; i++ {
		<-f.barC
		fmt.Print("bar")
		if i != f.n-1 {
			f.fooC <- struct{}{}
		}
		f.wg.Done()
	}
}

func main() {
	fb := FooBar{
		n:    5,
		fooC: make(chan struct{}),
		barC: make(chan struct{}),
		wg:   &sync.WaitGroup{},
	}

	fb.wg.Add(fb.n)
	go fb.foo()
	go fb.bar()

	fb.fooC <- struct{}{}
	fb.wg.Wait()
	close(fb.fooC)
	close(fb.barC)
}
