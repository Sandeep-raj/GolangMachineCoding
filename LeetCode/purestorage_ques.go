This is essentially an **event/dependency notification problem**: callers arrive at arbitrary times and want to run a callback when some one-time event occurs. The important property is that the event has **sticky/completed state**: once it has happened, future callers should observe that fact immediately rather than registering again.

 A clean solution is to model the event as a **one-shot promise/future/latch** with a callback list.

 ## 1\. Formalizing the problem

 Suppose we have an event `E` and callbacks:

```
registerCallback(f)
```

 The event eventually transitions:

```
NOT_FIRED  --->  FIRED
```

 For every callback `f`, we need:

 1. If `E` is already `FIRED` when `registerCallback(f)` is called:

   ```
   f()
   ```
    should happen immediately.
2. If `E` is still `NOT_FIRED`:

   ```
   f
   ```
    must be stored somewhere.
3. When `E` fires:
   - atomically change the state to `FIRED`;
   - obtain all registered callbacks;
   - execute each callback exactly once.
4. After that point, callbacks registered later must **not** be put back into the callback queue. They can simply be executed because the event is already complete.

 This gives us a very useful invariant:

 > **Once an event becomes fired, it never becomes unfired again.**

 That monotonic state transition makes the problem considerably easier.

---

 # 2\. Single-threaded solution

 In a single-threaded application, there is no concurrent access to the event state.

 We can therefore use:

```
Event {
    fired: boolean
    callbacks: list<Function>
}
```

 Initially:

```
fired = false
callbacks = []
```

 ## Registering a callback

 The algorithm is simply:

```
registerCallback(callback):
    if fired:
        callback()
    else:
        callbacks.append(callback)
```

 ## Firing the event

```
fire():
    if fired:
        return

    fired = true

    callbacks_to_run = callbacks
    callbacks = []

    for callback in callbacks_to_run:
        callback()
```

 The important step is setting:

```
fired = true
```

 **before** executing callbacks.

---

 ## Example

 Suppose:

```
registerCallback(A)
registerCallback(B)
fire()
registerCallback(C)
```

 Initially:

```
fired = false
callbacks = []
```

 After registering `A`:

```
fired = false
callbacks = [A]
```

 After registering `B`:

```
fired = false
callbacks = [A, B]
```

 Then `fire()` happens.

 We first do:

```
fired = true
```

 and extract the callbacks:

```
callbacks = []
callbacks_to_run = [A, B]
```

 Then:

```
A()
B()
```

 Now `C` is registered.

 Because:

```
fired == true
```

 we execute:

```
C()
```

 immediately.

 So we get:

```
A
B
C
```

 and `C` never needs to enter the callback list.

---

 # 3\. Why clearing the callback list matters

 Consider:

```
fire()
```

 After the event has fired, we don't want to keep references to every callback forever.

 For example:

```
callbacks = [A, B, C, D, ...]
```

 could become a memory leak if those functions capture large objects.

 Therefore, after transitioning to the completed state, detach the callback list:

```
callbacks_to_run = callbacks
callbacks = []
```

 Then execute the detached callbacks.

 This also gives us a useful conceptual separation:

```
Event state
    |
    +-- fired = true
    |
    +-- callbacks = []
```

 The event remembers only that it happened, not every callback that depended upon it.

---

 # 4\. A subtle question: what if a callback registers another callback?

 Suppose:

```
A() {
    registerCallback(B)
}
```

 and the event has already fired.

 Because `fired == true`, `B` is executed immediately.

 Thus:

```
fire()
    |
    +--> A()
           |
           +--> register(B)
                   |
                   +--> B()
```

 This works naturally.

 However, in a real application, you need to define what "immediately" means.

 It could mean:

```
B()
```

 is called synchronously from inside `registerCallback()`.

 Or it could mean:

```
queue(B)
```

 and execute it on the event loop.

 The latter is often safer in event-driven applications because it avoids arbitrarily deep callback recursion.

---

 # 5\. Exception handling in the single-threaded case

 Another issue is:

```
A() throws exception
```

 What happens to `B`, `C`, and `D`?

 You generally don't want:

```
A()
throws
```

 to prevent all other callbacks from running.

 A robust implementation can therefore do:

```
fire():
    if fired:
        return

    fired = true

    callbacks_to_run = callbacks
    callbacks = []

    for callback in callbacks_to_run:
        try:
            callback()
        catch exception:
            report(exception)
```

 The exact policy depends on the application.

 Possible policies include:

 - stop on the first exception;
- execute all callbacks and aggregate errors;
- report errors asynchronously;
- let the event-loop exception handler handle them.

 The key point is that **callback execution policy is separate from event-state management**.

---

 # 6\. Complexity of the single-threaded solution

 Let:

 - `N` = number of callbacks registered before the event fires.

 Then:

 ### Registration before firing

```
callbacks.append(callback)
```

 is generally:

```
O(1)
```

 amortized for an array-backed list.

 ### Registration after firing

```
if fired:
    callback()
```

 requires:

```
O(1)
```

 apart from the callback itself.

 ### Firing

 We must execute all `N` callbacks:

```
O(N)
```

 which is unavoidable because every callback has to run.

 ### Space

 Before firing:

```
O(N)
```

 After firing:

```
O(1)
```

 assuming the callback list is cleared.

---

 # 7\. Multi-threaded application

 The multi-threaded version is substantially more interesting.

 Now we can have:

```
Thread 1: registerCallback(A)

Thread 2: fire()

Thread 3: registerCallback(B)
```

 all happening concurrently.

 The major problem is a **race condition**.

 Consider:

```
Thread 1                         Thread 2

register(A)
    |
    | checks fired == false
    |
                                  fire()
                                    |
                                    +-- fired = true
                                    |
                                    +-- execute callbacks
    |
    +-- callbacks.add(A)
```

 Now `A` was registered **after the event was processed**, so it might never execute.

 That violates the requirement.

 The opposite ordering has the same problem:

```
Thread 1                         Thread 2

check fired == false
                                  fire()
                                  fired = true
                                  read callbacks
                                  callbacks = []
                                  execute
callbacks.add(A)
```

 Again, `A` is lost.

 Therefore we need to make the transition:

```
check event state
+
register callback
```

 an atomic operation with respect to:

```
fire event
```

---

 # 8\. Straightforward multi-threaded solution: mutex

 The simplest correct implementation uses a mutex/lock.

 Conceptually:

```
Event {
    mutex
    fired
    callbacks
}
```

 Registration:

```
registerCallback(callback):
    lock(mutex)

    if fired:
        unlock(mutex)
        callback()
    else:
        callbacks.append(callback)
        unlock(mutex)
```

 Firing:

```
fire():
    lock(mutex)

    if fired:
        unlock(mutex)
        return

    fired = true

    callbacks_to_run = callbacks
    callbacks = []

    unlock(mutex)

    for callback in callbacks_to_run:
        callback()
```

 This is the fundamental solution.

---

 # 9\. Why callbacks should NOT execute while holding the lock

 This is extremely important.

 A tempting implementation is:

```
fire():
    lock(mutex)

    fired = true

    for callback in callbacks:
        callback()

    unlock(mutex)
```

 This is generally a bad idea.

 Imagine:

```
callback A:
    registerCallback(X)
```

 Since `registerCallback()` needs the same mutex:

```
fire()
    holds mutex
       |
       +--> A()
              |
              +--> registerCallback(X)
                       |
                       +--> tries to acquire mutex
```

 We can deadlock if the mutex is non-reentrant.

 Even if the mutex is reentrant, we're holding the lock while executing arbitrary user code.

 That creates other problems:

 - callbacks may be slow;
- callbacks may block;
- callbacks may acquire other locks;
- callbacks may perform I/O;
- callbacks may recursively register more callbacks;
- lock contention becomes unnecessarily large.

 Therefore:

 > **Never execute arbitrary callbacks while holding the event's internal mutex.**

 Instead:

```
lock
    mark event fired
    detach callback list
unlock

execute callbacks
```

 This is a very important design pattern.

---

 # 10\. The critical section

 The critical section should be extremely small.

 For registration:

```
lock
    if fired:
        run_now = true
    else:
        callbacks.append(callback)
        run_now = false
unlock

if run_now:
    callback()
```

 For firing:

```
lock
    if already fired:
        return

    fired = true
    callbacks_to_run = callbacks
    callbacks = []
unlock

for callback in callbacks_to_run:
    callback()
```

 The lock protects the following invariant:

 > A callback must be either in the callback list **or** be selected for immediate execution, but never lost between those two states.

---

 # 11\. Race example with the correct implementation

 Consider:

```
Thread 1: register(A)
Thread 2: fire()
```

 Suppose Thread 1 obtains the lock first:

```
Thread 1:
lock
fired == false
callbacks = [A]
unlock
```

 Then Thread 2:

```
lock
fired = true
callbacks_to_run = [A]
callbacks = []
unlock

A()
```

 Correct.

 Now reverse the order.

 Thread 2 obtains the lock first:

```
Thread 2:
lock
fired = true
callbacks_to_run = []
callbacks = []
unlock
```

 Thread 1 then executes:

```
lock
fired == true
unlock

A()
```

 Also correct.

 The callback cannot disappear in either ordering.

---

 # 12\. The key atomicity requirement

 The most important part of the design is that these operations must be synchronized:

```
register:
    read fired
    potentially append callback
```

 and:

```
fire:
    set fired
    detach callbacks
```

 You need an atomic ordering between them.

 In other words, there must not be an intermediate state such as:

```
Thread A sees fired = false
Thread B sets fired = true
Thread A adds callback
```

 without B seeing A's callback.

 The mutex gives us that atomicity.

---

 # 13\. What does "execute immediately" mean in a multithreaded system?

 There's an interesting subtlety.

 Suppose Thread 1 calls:

```
registerCallback(A)
```

 after the event has fired.

 Should Thread 1 execute:

```
A()
```

 itself?

 The simplest answer is yes:

```
if fired:
    callback()
```

 That means the registering thread executes the callback.

 For example:

```
Thread 1
   |
   +-- register(A)
          |
          +-- sees fired
          |
          +-- A()
```

 This is usually perfectly acceptable.

 But there are alternative semantics.

 You could instead use an executor:

```
register(A)
    |
    +--> executor.submit(A)
```

 Then the callback is executed by a worker/event-loop thread.

 That can be useful when callbacks must execute in a particular execution context.

---

 # 14\. A better abstraction: one-shot event / promise

 The problem can be represented as:

```
OneShotEvent<T>
```

 or simply:

```
OneShotEvent
```

 with operations:

```
register(callback)
fire()
```

 It is essentially a primitive version of:

 - a future;
- a promise;
- a completion source;
- a one-shot latch;
- a deferred notification;
- a manually completed asynchronous result.

 For example:

```
Promise<T>
       |
       +---- pending
       |
       +---- completed(T)
```

 Anyone waiting for the promise can register a continuation.

 Once completed:

```
promise.then(A)
promise.then(B)
promise.then(C)
```

 can all execute without the underlying event happening again.

---

 # 15\. Multiple events and dependency chains

 The phrase:

 > "After the event has fired, any callback dependent on it can be executed without needing to be registered again."

 suggests that this primitive may be used to build dependency chains.

 For example:

```
Event A
   |
   +---- callback B
            |
            +---- Event B
                    |
                    +---- callback C
                             |
                             +---- Event C
```

 Or:

```
A ---> B ---> C ---> D
```

 If `A` has already completed, registering something dependent on `A` immediately allows the dependency chain to progress.

 This is essentially how futures/promises work.

---

 # 16\. A useful state-machine model

 We can formally define the event as:

```
              fire()
    PENDING -------------> FIRED
       |                     |
       | register            | register
       |                     |
       v                     v
 callback list            execute
```

 There is no are only two states:

```
PENDING
FIRED
```

 And only one legal transition:

```
PENDING -> FIRED
```

 There is no:

```
FIRED -> PENDING
```

 This is called a **monotonic state transition**.

 That property is extremely useful for concurrent programming.

---

 # 17\. Multi-threaded implementation using a lock

 Here is language-independent pseudocode:

```
class OneShotEvent:

    mutex
    fired = false
    callbacks = []

    registerCallback(callback):

        lock(mutex)

        if fired:
            unlock(mutex)

            callback()
            return

        callbacks.append(callback)

        unlock(mutex)

    fire():

        lock(mutex)

        if fired:
            unlock(mutex)
            return

        fired = true

        callbacks_to_run = callbacks
        callbacks = []

        unlock(mutex)

        for callback in callbacks_to_run:
            callback()
```

 This is already a correct solution for the basic problem.

---

 # 18\. But there is another concurrency concern: callback ordering

 Suppose:

```
Thread 1: register(A)
Thread 2: register(B)
```

 There may be no meaningful global ordering between A and B unless you explicitly define one.

 The mutex establishes an order in which registrations acquire the lock, but scheduling determines that order.

 If the application requires:

```
A before B
```

 then you need an explicit ordering mechanism.

 Otherwise the contract should simply be:

 > Every callback registered before the event is fired will execute exactly once, but callbacks registered concurrently have no guaranteed relative ordering.

 That's a much cleaner concurrency contract.

---

 # 19\. The harder question: how do we prevent registration from waiting excessively?

 Now to part **(a)**.

 The naive mutex solution has this structure:

```
registerCallback:
    acquire mutex
    modify callback list
    release mutex
```

 Normally the critical section is tiny, so this is fine.

 But suppose thousands of threads concurrently call:

```
registerCallback()
```

 They may contend for the mutex.

 The question asks:

 > How can we ensure that register callback functions do not wait excessively to queue in the callback?

 There are several approaches.

---

 # 20\. First principle: make the lock hold time tiny

 The most important optimization is simply:

 **Do not do expensive work while holding the lock.**

 Bad:

```
lock

callbacks.append(callback)

callback()          // BAD
database operation  // BAD
network operation   // BAD
logging operation   // potentially BAD

unlock
```

 Good:

```
lock

if fired:
    run_now = true
else:
    callbacks.append(callback)
    run_now = false

unlock

if run_now:
    callback()
```

 The critical section should contain essentially only:

```
check state
+
modify state
```

 This makes lock contention very short.

---

 # 21\. Separate registration from callback execution

 This distinction is crucial.

 The operation:

```
registerCallback()
```

 should ideally do only:

```
1. determine whether event has fired
2. enqueue callback OR determine that it should execute
```

 It should **not** actually perform expensive callback work while holding the event lock.

 For example:

```
register(A)
```

 should not mean:

```
lock
queue A
perform A
unlock
```

 Instead:

```
lock
queue A
unlock

// callback executes later
```

 This ensures another registration isn't blocked by A.

---

 # 22\. Use an executor / event queue

 For a highly concurrent system, a good architecture is:

```
                    +----------------+
registerCallback -->| callback queue |
                    +----------------+
                            |
                            v
                    +----------------+
                    | worker threads |
                    +----------------+
```

 Then:

```
register(A)
```

 only has to add `A` to the appropriate queue.

 Actual execution happens separately.

 For example:

```
registerCallback(A):

    lock
        if event not fired:
            callbacks.add(A)
        else:
            work_to_execute = A
    unlock

    if work_to_execute:
        executor.submit(A)
```

 The executor can then control:

 - number of worker threads;
- queue capacity;
- scheduling;
- prioritization;
- backpressure.

---

 # 23\. Important: don't submit to the executor while holding the event lock

 Even this can be problematic:

```
lock
    callbacks.add(A)
    executor.submit(A)
unlock
```

 `executor.submit()` may itself:

 - acquire another lock;
- allocate memory;
- block;
- reject work;
- perform bookkeeping.

 It is cleaner to make the event lock responsible only for event state.

 For example:

```
lock
    determine action
unlock

executor.submit(...)
```

 This gives us a very clean lock hierarchy.

---

 # 24\. A two-phase registration algorithm

 A robust pattern is:

```
registerCallback(callback):

    lock

    if fired:
        should_execute = true
    else:
        callbacks.append(callback)
        should_execute = false

    unlock

    if should_execute:
        executor.submit(callback)
```

 Now the lock is held only long enough to decide:

```
queue callback
```

 versus:

```
execute callback
```

 The expensive operation happens outside the lock.

---

 # 25\. What if the event fires while registration is occurring?

 This is where synchronization matters.

 Suppose:

```
Thread 1:
register(A)

Thread 2:
fire()
```

 The mutex establishes one of two possible orderings.

 ### Ordering 1

```
register(A)
```

 wins the lock first.

 Then:

```
callbacks = [A]
fired = false
```

 `fire()` subsequently takes the lock and extracts A.

 ### Ordering 2

```
fire()
```

 wins first.

 Then:

```
fired = true
```

 and registration subsequently sees:

```
fired == true
```

 and schedules A immediately.

 Therefore:

 > There is no lost callback.

---

 # 26\. Can we use a read-write lock?

 Potentially, but usually it doesn't buy much here.

 Registration performs a write when the event hasn't fired:

```
callbacks.append(callback)
```

 so many registrations cannot safely use the read side simultaneously.

 After the event fires, registration only needs to read:

```
fired
```

 but at that point the operation is trivial.

 A normal mutex is therefore often the best engineering choice.

 Don't introduce a more complicated synchronization primitive unless measurements show the mutex is a bottleneck.

---

 # 27\. Lock-free implementation

 If registration throughput is extremely important, you can consider a lock-free design.

 The core idea is to use an atomic state representing either:

```
PENDING + callback list
```

 or:

```
FIRED
```

 Then registration performs an atomic operation:

```
old_state = atomic_load(state)

if old_state == FIRED:
    execute callback
else:
    attempt CAS(old_state, old_state + callback)
```

 `CAS` means:

```
Compare-And-Swap
```

 Conceptually:

```
CAS(
    expected = old_state,
    desired = state_with_callback
)
```

 If another thread changes the state first, CAS fails and the registration retries.

---

 # 28\. Why lock-free gets complicated

 Suppose:

```
Thread A: register(A)
Thread B: fire()
```

 Both may simultaneously see:

```
state = PENDING
```

 Thread A tries to add callback A.

 Thread B tries to transition to:

```
FIRED
```

 Only one atomic state transition can win.

 If B wins:

```
state = FIRED
```

 A must observe that and execute its callback immediately.

 If A wins:

```
state = PENDING + [A]
```

 B then transitions that state to:

```
FIRED + [A]
```

 and executes A.

 This can be made correct, but implementing the callback list itself without locks introduces substantial complexity.

 You now have to think about:

 - ABA problems;
- memory reclamation;
- immutable lists;
- hazard pointers;
- epoch-based reclamation;
- atomic reference counting;
- allocation pressure;
- memory ordering;
- starvation.

 Therefore, unless registration throughput is genuinely a measured bottleneck, **a mutex is usually preferable**.

---

 # 29\. An alternative: atomic state + concurrent queue

 A practical compromise is to use:

```
AtomicBoolean fired
ConcurrentQueue callbacks
```

 But be careful.

 This naive code is **not sufficient**:

```
register(callback):
    if !fired:
        callbacks.add(callback)
    else:
        callback()
```

 because:

```
Thread A:
    sees fired == false

Thread B:
    fired = true
    drains queue

Thread A:
    callbacks.add(A)
```

 A is lost.

 So simply using a concurrent queue does not solve the coordination problem.

 The **state transition and queue insertion must be coordinated**.

 This is the fundamental difficulty.

---

 # 30\. A useful optimization: fast path after the event fires

 Notice that after:

```
fired = true
```

 the event never goes back.

 So registrations after completion are extremely common candidates for a fast path:

```
register(callback):

    if atomic_load(fired):
        execute(callback)
        return

    lock
       ...
```

 This can avoid acquiring the mutex for the overwhelmingly common post-event case.

 However, there's a subtle race:

```
Thread A:
    reads fired == false

Thread B:
    fires event

Thread A:
    enters lock
```

 That's okay **provided the locked path checks `fired` again**.

 For example:

```
register(callback):

    if fired:
        execute(callback)
        return

    lock

        if fired:
            should_execute = true
        else:
            callbacks.add(callback)
            should_execute = false

    unlock

    if should_execute:
        execute(callback)
```

 The first check is merely an optimization.

 The check inside the critical section provides correctness.

---

 # 31\. This gives us a very efficient implementation

 Conceptually:

```
registerCallback(callback):

    // Fast path
    if atomic_load(fired):
        executor.submit(callback)
        return

    lock

    // Recheck because event may have fired
    // after the fast-path check.
    if fired:
        run_now = true
    else:
        callbacks.append(callback)
        run_now = false

    unlock

    if run_now:
        executor.submit(callback)
```

 This is a very good practical design.

 Most registrations after the event fires take:

```
atomic read
+
executor submission
```

 and don't contend on the mutex.

---

 # 32\. Firing with the same optimization

 `fire()`:

```
fire():

    lock

    if fired:
        unlock
        return

    fired = true

    pending = callbacks
    callbacks = []

    unlock

    for callback in pending:
        executor.submit(callback)
```

 Again, no callback executes while holding the lock.

---

 # 33\. Fairness and excessive waiting

 The phrase "do not wait excessively" could also refer to **fairness**.

 A lock can theoretically allow one thread to repeatedly acquire the lock while another waits.

 If strict fairness is required, you could use a fair mutex or queue-based synchronization mechanism.

 Examples conceptually include:

```
FIFO mutex
ticket lock
MCS lock
```

 A ticket lock works approximately like:

```
Thread A gets ticket 10
Thread B gets ticket 11
Thread C gets ticket 12
```

 and they acquire the critical section in ticket order.

 This prevents a thread from repeatedly cutting ahead.

 However, fairness has a cost.

 For a callback registration operation where the critical section is tiny, a standard mutex is generally sufficient.

---

 # 34\. Backpressure is a separate issue

 Suppose 10 million callbacks are registered:

```
callbacks = [A1, A2, A3, ..., A10,000,000]
```

 Even if locking is perfect, eventually the event fires and you have to execute millions of callbacks.

 The problem is no longer lock contention.

 It is:

```
callback backlog
```

 So a production system may need:

 - bounded queues;
- worker pools;
- priorities;
- batching;
- cancellation;
- rate limiting;
- backpressure.

 For example:

```
Event
  |
  v
Callback Queue
  |
  +--> Worker 1
  +--> Worker 2
  +--> Worker 3
  +--> Worker 4
```

 Then the event firing operation itself can remain cheap:

```
mark fired
detach callbacks
enqueue callbacks
```

 while workers perform the expensive work.

---

 # 35\. Exactly-once execution

 The requirement says callbacks should execute once.

 There are actually two notions here.

 ### At-most-once

 The event implementation never intentionally invokes a callback more than once.

 ### Exactly-once

 The callback is guaranteed to complete exactly once even if:

 - a worker crashes;
- the process crashes;
- callback throws;
- executor rejects work;
- application restarts.

 The simple in-memory algorithm provides **at-most-once invocation**, assuming the program remains alive.

 It does **not** provide durable exactly-once execution across process crashes.

 If durable exactly-once semantics are required, this becomes a much larger distributed-systems problem involving persistent queues/transactions/idempotency.

 For the stated single-process problem, at-most-once invocation is normally the intended meaning.

---

 # 36\. Callback cancellation

 A real system may also need:

```
registerCallback(callback) -> registrationHandle
```

 where:

```
handle.cancel()
```

 removes the callback if the event hasn't fired.

 This adds another synchronization concern.

 Conceptually:

```
lock

if not fired:
    remove callback

unlock
```

 If cancellation races with firing, you need to define semantics.

 For example:

```
cancel wins
```

 means the callback won't run.

 Or:

```
fire wins
```

 means cancellation may fail if execution has already been selected.

 Again, the lock provides a clean linearization point.

---

 # 37\. Linearizability

 For the multi-threaded solution, it is useful to think in terms of **linearization points**.

 For:

```
registerCallback(A)
```

 the linearization point is:

```
lock-protected check/add
```

 For:

```
fire()
```

 the linearization point is:

```
fired = true
```

 Once that assignment occurs, conceptually the event has happened.

 Then every registration can be classified as either:

```
before the event
```

 or:

```
after the event
```

 even if the actual threads overlap in time.

 This makes the concurrency semantics precise.

---

 # 38\. Single-threaded versus multi-threaded comparison

 | Property | Single-threaded | Multi-threaded |
| --- | --- | --- |
| Event state | boolean | synchronized/atomic |
| Callback list | normal list | lock-protected list |
| Registration | simple | synchronized |
| Event firing | simple | synchronized transition |
| Race conditions | none | must handle |
| Lost callback possibility | no | yes, without synchronization |
| Lock required | no | simplest solution: yes |
| Callback execution under lock | irrelevant | should avoid |
| Fast path after firing | simple | atomic state check |
| Complexity | low | moderate |

---

 # 39\. Recommended production architecture

 For most applications, I would implement the design like this:

```
                 +---------------------+
                 |     OneShotEvent    |
                 +---------------------+
                 | atomic fired state  |
                 | mutex               |
                 | callback list       |
                 +----------+----------+
                            |
                +-----------+-----------+
                |                       |
          register()                 fire()
                |                       |
          acquire lock             acquire lock
                |                       |
          check fired              check fired
                |                       |
        +-------+-------+          set fired=true
        |               |                |
    pending          fired               |
        |               |          detach callbacks
   add to list          |                |
        |               |          release lock
   unlock               |                |
                        |          submit callbacks
                     unlock
                        |
                  execute/submit
```

 The key rule is:

 > **The event lock protects only the event's state and callback collection. It does not protect callback execution.**

---

 # 40\. Final pseudocode

 A polished version is:

```
class OneShotEvent:

    mutex
    atomic<bool> fired = false
    List<Callback> callbacks = []

    registerCallback(callback):

        # Fast path.
        # If the event has already fired, don't contend for the lock.
        if atomic_load(fired):
            executor.submit(callback)
            return

        # Slow path.
        lock(mutex)

        # The event could have fired between the first check
        # and acquiring the lock, so check again.
        if fired:
            run_now = true
        else:
            callbacks.append(callback)
            run_now = false

        unlock(mutex)

        # Never execute arbitrary user code under our lock.
        if run_now:
            executor.submit(callback)

    fire():

        lock(mutex)

        if fired:
            unlock(mutex)
            return

        # This is the linearization point.
        fired = true

        # Detach the list.
        pending = callbacks
        callbacks = []

        unlock(mutex)

        # Execute outside the lock.
        for callback in pending:
            executor.submit(callback)
```

---

 # 41\. Why this solves the original requirements

 Let's check every requirement.

 ### Case 1: event already happened

```
fired == true
```

 so:

```
registerCallback(A)
```

 takes the fast path and schedules `A`.

 No registration is necessary.

 ### Case 2: event hasn't happened

```
fired == false
```

 so:

```
callbacks.append(A)
```

 and `A` waits.

 When:

```
fire()
```

 occurs, `A` is detached and scheduled.

 ### Case 3: event has fired

 The state permanently remains:

```
fired == true
```

 so every future callback takes the already-fired path.

 ### Case 4: registration races with firing

 The mutex establishes an ordering between:

```
register
```

 and:

```
fire
```

 so the callback is either:

```
stored before firing
```

 or:

```
observes firing and is scheduled afterward
```

 It cannot disappear.

 ### Case 5: callback is slow

 The callback runs outside the event mutex.

 Therefore:

```
slow_callback()
```

 does not prevent another thread from calling:

```
registerCallback()
```

 ### Case 6: many registration threads

 The critical section is extremely small:

```
check state
+
append callback
```

 and after firing, the atomic fast path can avoid the mutex altogether.

---

 # 42\. Answer to part (a), specifically

 If the interviewer asks:

 > **How can we ensure that register callback functions do not wait excessively to queue in the callback?**

 A strong concise answer is:

 > Keep the synchronization critical section extremely short. The mutex should protect only the transition between "callback is pending" and "event has fired," plus insertion/removal from the callback list. Never execute callbacks while holding the mutex. Once the event has fired, use an atomic `fired` flag as a fast path so future registrations don't need to acquire the mutex at all. If callback execution itself is expensive, dispatch callbacks to an executor/thread pool after releasing the lock. If contention is proven to be a bottleneck under very high registration rates, a lock-free state/list implementation can be considered, but it adds significant complexity and should be justified by measurement.

---

 # 43\. Interview-style final answer

 If this were a system-design/interview question, I'd present it in this order:

 1. **Represent the event with two states:** `PENDING` and `FIRED`.
2. **Maintain a callback list** while the event is pending.
3. **Single-threaded:** check `fired`; execute immediately if true, otherwise append to the list.
4. **On fire:** set `fired = true`, detach the callback list, and execute all detached callbacks.
5. **Multi-threaded:** protect the state transition and callback-list modification with a mutex.
6. **Never execute callbacks while holding that mutex.**
7. **For registration after firing, use an atomic fast path** to avoid unnecessary lock contention.
8. **For expensive callbacks, submit them to an executor** after releasing the lock.
9. **For extreme contention**, consider lock-free/CAS-based structures, but only after profiling.
10. The crucial invariant is:

 > **Every callback is either inserted before the event's atomic transition to `FIRED`, or observes `FIRED` and is executed afterward. No callback can fall between those two cases.**

 That invariant is the heart of the entire solution.
