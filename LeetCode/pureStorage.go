// Online Go compiler to run Golang program online
// Print "Start small. Ship something." message

/*
Topic: Threads and Synchronization
Question: There are multiple users calling a method reg_cb at different intances of time, as shown below. Simultaneously, there is an event happening. All the user requests that were made during the execution of the event should wait till the event completes and then execute the reg_cb method. Once the event is finished, the user requests to the reg_cb method can be executed immediatly. Implement how to handle the given scenario.

					Event in progress
----|---------------|--------------------|-------------------|--------------> timeline
U1: reg_cb(f1)     U2: reg_cb(f2)      Event completed      U3:reg_cb(f3)
									   (execute f1,f2)      (execute f3)
Was asked many questions on basic fundamentals like,

When does a concurrant modification exception occur?
When is the possibility of same thread (user x) calling the reg_cb() twice?
What are the possible deadlock scenarios?
What is mutex? etc..
Expectations: Concentrate on how you handle different possible scenarios with a valid scenario and explanation. Wrinting code is secondary.
*/

package main
import (
  "fmt"
  "time"
  "sync"
  )

type cb func() 
var mu sync.RWMutex = sync.RWMutex{}

func regCb(fn cb) {
  mu.RLock()
  defer mu.RUnlock()
  fmt.Println("executing cb started")
  fn();
  fmt.Println("cb excuted. exiting...")
}

func execEvents() {
  mu.Lock()
  defer mu.Unlock()
  fmt.Println("executing event started")
  time.Sleep(5 * time.Second);
  fmt.Println("event excuted. exiting...")
}

func main() {
  fmt.Println("Start small. Ship something.")
  wg := sync.WaitGroup{}

  wg.Add(1)
  go func() {
    defer wg.Done()
    regCb(func() {
      time.Sleep(2 * time.Second);
    })
  }()

  wg.Add(1)
  go func() {
    defer wg.Done()
    execEvents()
  }()

  time.Sleep(1 * time.Second)
  wg.Add(1)
  go func() {
    defer wg.Done()
    regCb(func() {
      time.Sleep(2 * time.Second);
    })
  }()

  time.Sleep(2 * time.Second)
  wg.Add(1)
  go func() {
    defer wg.Done()
    regCb(func() {
      time.Sleep(2 * time.Second);
    })
  }()

  wg.Wait()
  fmt.Println("Workflow Completed")
}
