package multithreading

import (
	"sync"
)

// ThreadTracker allows you to easily manage goroutines
type ThreadTracker struct {
	activeThreadsCounter *sync.WaitGroup
}

// MakeThreadTracker is a factory method for ThreadTracker
func MakeThreadTracker() *ThreadTracker {
	return &ThreadTracker{
		activeThreadsCounter: &sync.WaitGroup{},
	}
}

// TriggerGoroutine initiates a new goroutine while tracking it
// typical usage 1:
//     threadTracker.TriggerGoroutine(func(inputs []interface{}) {
//         fmt.Printf("Hello %s\n", "World")
//     })
// typical usage 2:
//     threadTracker.TriggerGoroutine(func(inputs []interface{}) {
//         fmt.Printf("Hello %s\n", inputs[0])
//     }, "World")
func (t *ThreadTracker) TriggerGoroutine(fn func(inputs []interface{}), inputs []interface{}) {
	t.TriggerGoroutineWithDefers(nil, fn, inputs)
}

// TriggerGoroutineWithDefers initiates a new goroutine while tracking it
// typical usage:
//     threadTracker.TriggerGoroutineWithDefers([]func(){
// 	       func() { fmt.Printf("this should appear third\n") },
// 	       func() { fmt.Printf("this should appear second\n") },
//     }, func(inputs []interface{}) {
//         fmt.Printf("Hello %s -- this should appear first\n", "World")
//     })
func (t *ThreadTracker) TriggerGoroutineWithDefers(deferredFns []func(), fn func(inputs []interface{}), inputs []interface{}) {
	t.activeThreadsCounter.Add(1)

	go func() {
		// defer this func so it is called even if the code panics
		defer func() {
			t.activeThreadsCounter.Done()
		}()
		if deferredFns != nil {
			for _, dFn := range deferredFns {
				defer dFn()
			}
		}

		fn(inputs)
	}()
}

// Wait blocks until all goroutines finish
func (t *ThreadTracker) Wait() {
	t.activeThreadsCounter.Wait()
}
