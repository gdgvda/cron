package cron

import (
	"container/heap"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Cron keeps track of any number of entries, invoking the associated func as
// specified by the schedule. It may be started, stopped, and the entries may
// be inspected while running.
type Cron struct {
	entries   entryHeap
	chain     Chain
	stop      chan struct{}
	add       chan *Entry
	remove    chan ID
	snapshot  chan chan []Entry
	running   bool
	logger    *slog.Logger
	runningMu sync.Mutex
	location  *time.Location
	parser    Parser
	next      ID
	jobWaiter sync.WaitGroup
}

// Schedule describes a job's duty cycle.
type Schedule interface {
	// Next returns the next activation time, later than the given time.
	// Next is invoked initially, and then each time the job is run.
	Next(time.Time) time.Time
}

// ID identifies an entry within a Cron instance
type ID uint

// Entry consists of a schedule and the func to execute on that schedule.
type Entry struct {
	// ID is the cron-assigned ID of this entry, which may be used to look up a
	// snapshot or remove it.
	ID ID

	// Schedule on which this job should be run.
	Schedule Schedule

	// Next time the job will run, or the zero time if Cron has not been
	// started or this entry's schedule is unsatisfiable
	Next time.Time

	// Prev is the last time this job was run, or the zero time if never.
	Prev time.Time

	job    func()
	logger *slog.Logger
}

// New returns a new Cron job runner, modified by the given options.
//
// Available Settings
//
//	Time Zone
//	  Description: The time zone in which schedules are interpreted
//	  Default:     time.Local
//
//	Parser
//	  Description: Parser converts cron spec strings into cron.Schedules.
//	  Default:     Accepts this spec: https://en.wikipedia.org/wiki/Cron
//
//	Chain
//	  Description: Wrap submitted jobs to customize behavior.
//	  Default:     A chain that recovers panics and logs them to stderr.
//
// See "cron.With*" to modify the default behavior.
func New(opts ...Option) *Cron {
	c := &Cron{
		entries:   entryHeap{},
		chain:     NewChain(),
		add:       make(chan *Entry),
		stop:      make(chan struct{}),
		snapshot:  make(chan chan []Entry),
		remove:    make(chan ID),
		running:   false,
		runningMu: sync.Mutex{},
		logger:    slog.Default(),
		location:  time.Local,
		parser:    standardParser,
		next:      1,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Add adds a job to the Cron to be run on the given schedule.
// The spec is parsed using the time zone of this Cron instance as the default.
// An opaque ID is returned that can be used to later remove it.
func (c *Cron) Add(spec string, cmd func()) (ID, error) {
	schedule, err := c.parser.Parse(spec)
	if err != nil {
		return 0, err
	}
	return c.Schedule(schedule, cmd)
}

// Schedule adds a job to the Cron to be run on the given schedule.
// The job is wrapped with the configured Chain.
func (c *Cron) Schedule(schedule Schedule, cmd func()) (ID, error) {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()

	if c.next == 0 {
		return 0, fmt.Errorf("run out of available ids")
	}

	entry := &Entry{
		ID:       c.next,
		Schedule: schedule,
		job:      c.chain.Then(cmd),
		logger:   c.logger.With("id", c.next),
	}
	c.next++
	if !c.running {
		c.entries = append(c.entries, entry)
	} else {
		c.add <- entry
	}
	return entry.ID, nil
}

// Entries returns a snapshot of the cron entries.
func (c *Cron) Entries() []Entry {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if c.running {
		replyChan := make(chan []Entry, 1)
		c.snapshot <- replyChan
		return <-replyChan
	}
	return c.entrySnapshot()
}

// Location gets the time zone location
func (c *Cron) Location() *time.Location {
	return c.location
}

// Entry returns a snapshot of the given entry, or nil if it couldn't be found.
func (c *Cron) Entry(id ID) Entry {
	for _, entry := range c.Entries() {
		if id == entry.ID {
			return entry
		}
	}
	return Entry{}
}

// Remove an entry from being run in the future.
func (c *Cron) Remove(id ID) {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if c.running {
		c.remove <- id
	} else {
		c.removeEntry(id)
	}
}

// Start the cron scheduler in its own goroutine, or no-op if already started.
func (c *Cron) Start() {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if c.running {
		return
	}
	c.running = true
	go c.run()
}

// Run the cron scheduler, or no-op if already running.
func (c *Cron) Run() {
	c.runningMu.Lock()
	if c.running {
		c.runningMu.Unlock()
		return
	}
	c.running = true
	c.runningMu.Unlock()
	c.run()
}

// run the scheduler.. this is private just due to the need to synchronize
// access to the 'running' state variable.
func (c *Cron) run() {
	c.logger.Info("starting scheduler", "event", "start")

	// Figure out the next activation times for each entry.
	now := c.now()
	for _, entry := range c.entries {
		entry.Next = entry.Schedule.Next(now)
		entry.logger.Debug("next execution time computed", "event", "next", "now", now, "next", entry.Next)
	}
	heap.Init(&c.entries)

	for {
		var timer *time.Timer
		if len(c.entries) == 0 || c.entries[0].Next.IsZero() {
			// If there are no entries yet, just sleep - it still handles new entries
			// and stop requests.
			timer = time.NewTimer(100000 * time.Hour)
		} else {
			timer = time.NewTimer(c.entries[0].Next.Sub(now))
		}

		for {
			select {
			case now = <-timer.C:
				now = now.In(c.location)
				c.logger.Debug("scheduler woke up", "event", "wake", "now", now)

				// Run every entry whose next time was less than now
				for {
					if c.entries[0].Next.After(now) || c.entries[0].Next.IsZero() {
						break
					}
					e := heap.Pop(&c.entries).(*Entry)
					c.startJob(e.job)
					e.Prev = e.Next
					e.Next = e.Schedule.Next(now)
					heap.Push(&c.entries, e)
					e.logger.Info("starting job", "event", "run", "now", now, "next", e.Next)
				}

			case newEntry := <-c.add:
				timer.Stop()
				now = c.now()
				newEntry.Next = newEntry.Schedule.Next(now)
				heap.Push(&c.entries, newEntry)
				newEntry.logger.Info("added new entry", "event", "add", "now", now, "next", newEntry.Next)

			case replyChan := <-c.snapshot:
				replyChan <- c.entrySnapshot()
				continue

			case <-c.stop:
				timer.Stop()
				c.logger.Info("stopping scheduler", "event", "stop")
				return

			case id := <-c.remove:
				timer.Stop()
				now = c.now()
				c.removeEntry(id)
			}

			break
		}
	}
}

// startJob runs the given job in a new goroutine.
func (c *Cron) startJob(job func()) {
	c.jobWaiter.Add(1)
	go func() {
		defer c.jobWaiter.Done()
		job()
	}()
}

// now returns current time in c location
func (c *Cron) now() time.Time {
	return time.Now().In(c.location)
}

// Stop stops the cron scheduler if it is running; otherwise it does nothing.
// A context is returned so the caller can wait for running jobs to complete.
func (c *Cron) Stop() context.Context {
	c.runningMu.Lock()
	defer c.runningMu.Unlock()
	if c.running {
		c.stop <- struct{}{}
		c.running = false
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c.jobWaiter.Wait()
		cancel()
	}()
	return ctx
}

// entrySnapshot returns a copy of the current cron entry list.
func (c *Cron) entrySnapshot() []Entry {
	var entries = make([]Entry, len(c.entries))
	for i, e := range c.entries {
		entries[i] = *e
	}
	return entries
}

func (c *Cron) removeEntry(id ID) {
	for idx, e := range c.entries {
		if e.ID == id {
			e.logger.Info("removed entry", "event", "remove")
			heap.Remove(&c.entries, idx)
			return
		}
	}
}
