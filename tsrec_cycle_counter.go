package cron

import "sync"

// tsrecCycleCounter is a concurrency-safe counter of the cycles whose jobs have been started but not
// yet completed.
type tsrecCycleCounter struct {
	mu    sync.Mutex
	count int
	// none is broadcast whenever count drops to zero.
	none *sync.Cond
}

func newTSRECCycleCounter() *tsrecCycleCounter {
	c := &tsrecCycleCounter{}
	c.none = sync.NewCond(&c.mu)
	return c
}

// started counts a cycle whose jobs are about to start.
func (c *tsrecCycleCounter) started() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
}

// completed counts a previously started cycle whose jobs have all finished.
func (c *tsrecCycleCounter) completed() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count--
	if c.count == 0 {
		c.none.Broadcast()
	}
}

// running returns the number of cycles currently running.
func (c *tsrecCycleCounter) running() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

// awaitNone blocks until no cycle is running.
func (c *tsrecCycleCounter) awaitNone() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for c.count > 0 {
		c.none.Wait()
	}
}
