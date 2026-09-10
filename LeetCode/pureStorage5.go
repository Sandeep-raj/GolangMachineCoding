package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// SINGLE-THREADED ONE-SHOT EVENT
// ============================================================

type SingleThreadedEvent struct {
	fired     bool
	callbacks []func()
}

func NewSingleThreadedEvent() *SingleThreadedEvent {
	return &SingleThreadedEvent{
		callbacks: make([]func(), 0),
	}
}

// RegisterCallback:
//
// If the event already fired:
//
//	execute callback immediately.
//
// Otherwise:
//
//	save callback for later.
func (e *SingleThreadedEvent) RegisterCallback(callback func()) {
	if e.fired {
		callback()
		return
	}

	e.callbacks = append(e.callbacks, callback)
}

// Fire:
//
// 1. Check whether already fired.
// 2. Mark event as fired.
// 3. Detach callback list.
// 4. Execute callbacks.
//
// Since this is single-threaded, no mutex is necessary.
func (e *SingleThreadedEvent) Fire() {
	if e.fired {
		return
	}

	// Important:
	// Mark the event as fired BEFORE executing callbacks.
	e.fired = true

	// Detach callbacks from the event.
	pending := e.callbacks
	e.callbacks = nil

	// Execute callbacks.
	for _, callback := range pending {
		callback()
	}
}

// ============================================================
// MULTI-THREADED ONE-SHOT EVENT
// ============================================================

type MultiThreadedEvent struct {
	// 0 = event has not fired
	// 1 = event has fired
	fired atomic.Uint32

	// Protects the callback list and the pending -> fired
	// transition.
	mutex sync.Mutex

	callbacks []func()

	// Executes callbacks outside the event mutex.
	executor func(func())
}

func NewMultiThreadedEvent() *MultiThreadedEvent {
	return &MultiThreadedEvent{
		callbacks: make([]func(), 0),

		executor: func(callback func()) {
			go callback()
		},
	}
}

// RegisterCallback:
//
// Fast path:
//
//	event already fired
//
// No mutex is needed.
//
// Slow path:
//
//	event might still be pending
//
// Acquire mutex and CHECK AGAIN.
//
// The second check is essential for correctness.
func (e *MultiThreadedEvent) RegisterCallback(callback func()) {

	// ========================================================
	// FAST PATH
	// ========================================================
	//
	// If event is already fired, avoid mutex contention.
	//
	if e.fired.Load() == 1 {
		e.executor(callback)
		return
	}

	// ========================================================
	// SLOW PATH
	// ========================================================

	e.mutex.Lock()

	// IMPORTANT:
	//
	// The event could have fired after our first Load()
	// but before we acquired the mutex.
	//
	// Therefore we MUST check again.
	if e.fired.Load() == 1 {
		e.mutex.Unlock()

		// Never execute arbitrary callback while holding mutex.
		e.executor(callback)
		return
	}

	// Event has not fired yet.
	//
	// Store callback.
	e.callbacks = append(e.callbacks, callback)

	e.mutex.Unlock()
}

// Fire:
//
// The critical section is intentionally tiny.
//
// We:
//
//	lock
//	mark fired
//	detach callbacks
//	unlock
//
// Then execute callbacks OUTSIDE the lock.
func (e *MultiThreadedEvent) Fire() {

	e.mutex.Lock()

	// Fire is idempotent.
	if e.fired.Load() == 1 {
		e.mutex.Unlock()
		return
	}

	// ========================================================
	// LINEARIZATION POINT
	// ========================================================
	//
	// From this point onward, the event is considered fired.
	e.fired.Store(1)

	// Detach pending callbacks.
	pending := e.callbacks
	e.callbacks = nil

	e.mutex.Unlock()

	// ========================================================
	// Execute callbacks OUTSIDE mutex
	// ========================================================

	for _, callback := range pending {
		e.executor(callback)
	}
}

// ============================================================
// SINGLE-THREADED SIMULATION
// ============================================================

func simulateSingleThreaded() {

	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("SIMULATION 1: SINGLE-THREADED")
	fmt.Println("============================================================")

	event := NewSingleThreadedEvent()

	fmt.Println()
	fmt.Println("1. Registering A before event fires")

	event.RegisterCallback(func() {
		fmt.Println("[A] executed")
	})

	fmt.Println("2. Registering B before event fires")

	event.RegisterCallback(func() {
		fmt.Println("[B] executed")
	})

	fmt.Println()
	fmt.Println("3. Firing event")

	event.Fire()

	fmt.Println()
	fmt.Println("4. Registering C AFTER event has fired")

	event.RegisterCallback(func() {
		fmt.Println("[C] executed immediately")
	})

	fmt.Println()
	fmt.Println("5. Calling Fire() again")

	event.Fire()

	fmt.Println()
	fmt.Println("Single-threaded simulation complete.")
}

// ============================================================
// MULTI-THREADED SIMULATION
// ============================================================

func simulateMultiThreaded() {

	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("SIMULATION 2: MULTI-THREADED")
	fmt.Println("============================================================")

	event := NewMultiThreadedEvent()

	// --------------------------------------------------------
	// registrationWG:
	//
	// Waits for goroutines performing registration and firing.
	// --------------------------------------------------------

	var registrationWG sync.WaitGroup

	// --------------------------------------------------------
	// callbackWG:
	//
	// Waits for callbacks themselves.
	// --------------------------------------------------------

	var callbackWG sync.WaitGroup

	const numberOfCallbacks = 10

	// 10 registration goroutines
	registrationWG.Add(numberOfCallbacks)

	// One Fire() goroutine
	registrationWG.Add(1)

	// One post-fire registration goroutine
	registrationWG.Add(1)

	// --------------------------------------------------------
	// Start registrations
	// --------------------------------------------------------

	fmt.Println()
	fmt.Println("Starting concurrent registrations...")

	for i := 1; i <= numberOfCallbacks; i++ {

		i := i

		go func() {

			defer registrationWG.Done()

			// Stagger registrations so that some happen before
			// Fire() and some happen after Fire().
			time.Sleep(time.Duration(i*30) * time.Millisecond)

			name := fmt.Sprintf("pre-fire-%d", i)

			fmt.Printf(
				"[%s] registering at %s\n",
				name,
				time.Now().Format("15:04:05.000"),
			)

			callbackWG.Add(1)

			event.RegisterCallback(func() {

				defer callbackWG.Done()

				fmt.Printf(
					"[%s] callback START at %s\n",
					name,
					time.Now().Format("15:04:05.000"),
				)

				// Simulate callback work.
				time.Sleep(100 * time.Millisecond)

				fmt.Printf(
					"[%s] callback END   at %s\n",
					name,
					time.Now().Format("15:04:05.000"),
				)
			})
		}()
	}

	// --------------------------------------------------------
	// Fire event
	// --------------------------------------------------------

	go func() {

		defer registrationWG.Done()

		// Fire after 200 ms.
		time.Sleep(200 * time.Millisecond)

		fmt.Printf(
			"\n[FIRE] Event firing at %s\n",
			time.Now().Format("15:04:05.000"),
		)

		event.Fire()

		fmt.Printf(
			"[FIRE] Event.Fire() returned at %s\n\n",
			time.Now().Format("15:04:05.000"),
		)
	}()

	// --------------------------------------------------------
	// Post-fire registrations
	// --------------------------------------------------------

	go func() {

		defer registrationWG.Done()

		// Definitely after Fire().
		time.Sleep(500 * time.Millisecond)

		for i := 1; i <= 3; i++ {

			name := fmt.Sprintf("post-fire-%d", i)

			fmt.Printf(
				"[%s] registering AFTER event at %s\n",
				name,
				time.Now().Format("15:04:05.000"),
			)

			callbackWG.Add(1)

			event.RegisterCallback(func() {

				defer callbackWG.Done()

				fmt.Printf(
					"[%s] executed immediately after registration\n",
					name,
				)
			})

			time.Sleep(50 * time.Millisecond)
		}
	}()

	// --------------------------------------------------------
	// Wait for registration/fire goroutines.
	// --------------------------------------------------------

	fmt.Println()
	fmt.Println("Main: waiting for registration/fire goroutines...")

	registrationWG.Wait()

	fmt.Println("Main: all registration/fire goroutines finished.")

	// --------------------------------------------------------
	// Now wait for actual callbacks.
	// --------------------------------------------------------

	fmt.Println("Main: waiting for callbacks...")

	callbackWG.Wait()

	fmt.Println("Main: all callbacks finished.")

	fmt.Println()
	fmt.Println("Multi-threaded simulation complete.")
}

// ============================================================
// RACE BETWEEN REGISTER AND FIRE
// ============================================================

func simulateRace() {

	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("SIMULATION 3: REGISTER VS FIRE RACE")
	fmt.Println("============================================================")

	event := NewMultiThreadedEvent()

	var wg sync.WaitGroup

	wg.Add(2)

	// --------------------------------------------------------
	// Thread 1: register
	// --------------------------------------------------------

	go func() {

		defer wg.Done()

		fmt.Println("[Thread 1] registering callback")

		event.RegisterCallback(func() {
			fmt.Println("[Thread 1] callback executed")
		})

		fmt.Println("[Thread 1] registration finished")

	}()

	// --------------------------------------------------------
	// Thread 2: fire
	// --------------------------------------------------------

	go func() {

		defer wg.Done()

		time.Sleep(1 * time.Millisecond)

		fmt.Println("[Thread 2] firing event")

		event.Fire()

		fmt.Println("[Thread 2] fire finished")

	}()

	wg.Wait()

	// --------------------------------------------------------
	// Registration after race
	// --------------------------------------------------------

	fmt.Println()
	fmt.Println("Registering callback after race is complete...")

	event.RegisterCallback(func() {
		fmt.Println("[Post-race] callback executed immediately")
	})

	// Give goroutine executor time to run.
	time.Sleep(100 * time.Millisecond)

	fmt.Println()
	fmt.Println("Race simulation complete.")
}

// ============================================================
// SLOW CALLBACK DEMONSTRATION
// ============================================================

func simulateSlowCallbacks() {

	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("SIMULATION 4: SLOW CALLBACK DOES NOT HOLD EVENT LOCK")
	fmt.Println("============================================================")

	event := NewMultiThreadedEvent()

	// --------------------------------------------------------
	// Register slow callback BEFORE event fires.
	// --------------------------------------------------------

	event.RegisterCallback(func() {

		fmt.Println(
			"[SLOW] callback START:",
			time.Now().Format("15:04:05.000"),
		)

		// Simulate expensive work.
		time.Sleep(2 * time.Second)

		fmt.Println(
			"[SLOW] callback END:",
			time.Now().Format("15:04:05.000"),
		)
	})

	fmt.Println()
	fmt.Println("Calling Fire()...")

	start := time.Now()

	event.Fire()

	elapsed := time.Since(start)

	fmt.Printf(
		"Fire() returned after %v\n",
		elapsed,
	)

	// --------------------------------------------------------
	// Event is already fired.
	//
	// Registration should NOT wait for the slow callback.
	// --------------------------------------------------------

	fmt.Println()
	fmt.Println("Registering FAST callback after Fire()...")

	start = time.Now()

	event.RegisterCallback(func() {

		fmt.Println(
			"[FAST] callback executed:",
			time.Now().Format("15:04:05.000"),
		)
	})

	fmt.Printf(
		"RegisterCallback() returned after %v\n",
		time.Since(start),
	)

	// Give slow callback enough time to finish.
	time.Sleep(2500 * time.Millisecond)

	fmt.Println()
	fmt.Println("Slow callback simulation complete.")
}

// ============================================================
// MAIN
// ============================================================

func main() {

	fmt.Println("One-Shot Event / Callback Demonstration")
	fmt.Println("========================================")

	// --------------------------------------------------------
	// 1. Single-threaded implementation
	// --------------------------------------------------------

	simulateSingleThreaded()

	// --------------------------------------------------------
	// 2. Multi-threaded implementation
	// --------------------------------------------------------

	simulateMultiThreaded()

	// --------------------------------------------------------
	// 3. Register vs Fire race
	// --------------------------------------------------------

	simulateRace()

	// --------------------------------------------------------
	// 4. Slow callback / lock contention demonstration
	// --------------------------------------------------------

	simulateSlowCallbacks()

	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("ALL SIMULATIONS COMPLETE")
	fmt.Println("============================================================")
}
