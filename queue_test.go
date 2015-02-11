package callqueue

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/bradfitz/iter"
)

func TestQueueSerializationAndCleanup(t *testing.T) {
	var q Queue
	var wg sync.WaitGroup
	var callers int
	f := func() {
		callers++
		runtime.Gosched()
		if callers != 1 {
			t.Error("multiple goroutines called f at the same time")
		}
		callers--
	}
	n := 100
	wg.Add(n)
	for _ = range iter.N(n) {
		go func() {
			q.Do("testing", f)
			wg.Done()
		}()
	}
	wg.Wait()
	if len(q.m) != 0 {
		t.Error("failed to clean up after running functions")
	}
}

func TestPanic(t *testing.T) {
	// Pass a function that panics to queue.Do.
	// Check that the queue reacts appropriately -- releasing locks and cleaning up resources.
	var q Queue
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expected a panic, but it didn't happen.")
		}
		if len(q.m) != 0 {
			t.Error("When a function passed to queue.Do panicked, the queue failed to clean up resources.")
		}
		c := make(chan bool)
		go func() {
			q.Do("key", func() {
				c <- true
			})
		}()
		select {
		case <-c:
		case <-time.After(100 * time.Millisecond):
			t.Error("When a function passed to queue.Do panicked, the queue failed to release the lock for that key.")
		}
	}()
	q.Do("key", func() {
		panic("testing")
	})
}
