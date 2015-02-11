package callqueue

import "sync"

// Queue represents a class of work and forms a namespace in which units of work can be serialized by key.
type Queue struct {
	mu sync.Mutex       // protects m
	m  map[string]*call // lazily initialized
}

// Size returns the number of keys currently locked in the queue. This is directly proportional to the
// size of the queue's underlying datastructure, independent of how many goroutines are waiting for
// those locks. This is also equal to the number of queued calls that are currently either executing
// (not waiting to execute) or releasing locks just after completing execution.
func (q *Queue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.m)
}

// Do executes the given function, making sure that only one execution runs at a time for a given key.
// If new calls come in for the same key, they wait for the caller at the head of the queue to finish
// before running.
func (q *Queue) Do(key string, fn func()) {
	c := q.get(key)
	defer q.release(key, c)
	c.mu.Lock()
	defer c.mu.Unlock()
	fn()
}

func (q *Queue) get(key string) *call {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.m == nil {
		q.m = make(map[string]*call)
	}
	c, ok := q.m[key]
	if !ok {
		c = new(call)
		q.m[key] = c
	}
	c.waiters++
	return c
}

func (q *Queue) release(key string, c *call) {
	q.mu.Lock()
	defer q.mu.Unlock()
	c.waiters--
	if c.waiters == 0 {
		delete(q.m, key)
	}
}

type call struct {
	waiters int
	mu      sync.Mutex
}
