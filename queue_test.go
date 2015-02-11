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
	for range iter.N(n) {
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

func TestSize(t *testing.T) {
	var q Queue
	quit := make(chan struct{})
	var wg sync.WaitGroup

	start := func(key string) {
		wg.Add(1)
		started := make(chan bool)
		go func() {
			q.Do(key, func() {
				started <- true
				<-quit
			})
			wg.Done()
		}()
		<-started
	}

	startNoBlock := func(key string) {
		wg.Add(1)
		go func() {
			q.Do(key, func() { <-quit })
			wg.Done()
		}()
	}

	sizeShouldBe := func(w int) {
		if g := q.Size(); g != w {
			_, _, line, _ := runtime.Caller(1)
			t.Errorf("At line %d, q.Size() == %d, expected %d.", line, g, w)
		}
	}

	sizeShouldBe(0)

	start("A")
	sizeShouldBe(1)

	startNoBlock("A") // Will wait (in another goroutine) for the previous function to end.
	sizeShouldBe(1)   // We count number of distinct keys in the queue, not number of functions waiting + executing.

	start("B")
	sizeShouldBe(2)

	start("C")
	start("D")
	sizeShouldBe(4)

	close(quit)
	wg.Wait()

	sizeShouldBe(0)
}

func TestSizeRace(t *testing.T) {
	var q Queue
	// This test provides no synchronization between Do and Size. If Queue doesn't either, go test -race should catch it.
	go q.Do("key", func() {})
	q.Size()
}
