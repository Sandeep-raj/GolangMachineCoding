package main

import (
	"log"
	"sync"
)

/*
1117. Building H2O

There are two kinds of threads: oxygen and hydrogen. Your goal is to group these threads to form water molecules.

There is a barrier where each thread has to wait until a complete molecule can be formed. Hydrogen and oxygen threads will be given releaseHydrogen and releaseOxygen methods respectively, which will allow them to pass the barrier. These threads should pass the barrier in groups of three, and they must immediately bond with each other to form a water molecule. You must guarantee that all the threads from one molecule bond before any other threads from the next molecule do.

In other words:

If an oxygen thread arrives at the barrier when no hydrogen threads are present, it must wait for two hydrogen threads.
If a hydrogen thread arrives at the barrier when no other threads are present, it must wait for an oxygen thread and another hydrogen thread.
We do not have to worry about matching the threads up explicitly; the threads do not necessarily know which other threads they are paired up with. The key is that threads pass the barriers in complete sets; thus, if we examine the sequence of threads that bind and divide them into groups of three, each group should contain one oxygen and two hydrogen threads.

Write synchronization code for oxygen and hydrogen molecules that enforces these constraints.



Example 1:

Input: water = "HOH"
Output: "HHO"
Explanation: "HOH" and "OHH" are also valid answers.
Example 2:

Input: water = "OOHHHH"
Output: "HHOHHO"
Explanation: "HOHHHO", "OHHHHO", "HHOHOH", "HOHHOH", "OHHHOH", "HHOOHH", "HOHOHH" and "OHHOHH" are also valid answers.
*/

type H2O struct {
	n      int
	hCount int
	oCount int
	cond   sync.Cond
	m      *sync.Mutex
}

func NewH2O() *H2O {
	lock := sync.Mutex{}
	h := &H2O{
		cond: *sync.NewCond(&lock),
		m:    &lock,
	}
	return h
}

func (h *H2O) Hydrogen() {
	h.m.Lock()

	for h.hCount >= 2 && h.oCount < 1 {
		h.cond.Wait()
	}

	h.hCount++
	log.Print("H")

	if h.hCount >= 2 && h.oCount >= 1 {
		h.hCount = h.hCount - 2
		h.oCount = h.oCount - 1

		h.cond.Broadcast()
	}

	h.m.Unlock()
}

func (h *H2O) Oxygen() {
	h.m.Lock()

	for h.hCount < 2 && h.oCount >= 1 {
		h.cond.Wait()
	}

	h.oCount++
	log.Print("O")

	if h.hCount >= 2 && h.oCount >= 1 {
		h.hCount = h.hCount - 2
		h.oCount = h.oCount - 1

		h.cond.Broadcast()
	}

	h.m.Unlock()
}

func main() {
	h2o := NewH2O()
	atoms := "HOOHHHOOHHHH"
	wg := sync.WaitGroup{}

	wg.Add(len(atoms))

	for _, c := range atoms {
		switch c {
		case 'H':
			go func() {
				defer wg.Done()
				h2o.Hydrogen()
			}()
		case 'O':
			go func() {
				defer wg.Done()
				h2o.Oxygen()
			}()
		}
	}

	wg.Wait()
}
