/*
Package cron implements a cron spec parser and job runner.

The library is organized into two primary components:

  - Parser: parses cron specification strings into a Schedule. Custom parsers can be
    created to support alternative formats.
  - Scheduler: the Cron instance keeps track of jobs and executes them according to
    their Schedule.

This separation allows parsing schedules independently, reusing a single schedule
for multiple jobs, or providing custom Schedule implementations directly to the scheduler.

# Usage

Callers may register funcs to be invoked on a given schedule. Cron will run
them in their own goroutines.

	c := cron.New()

	sched, _ := cron.ParseStandard("30 * * * *")
	c.Schedule(sched, func() { fmt.Println("Every hour on the half hour") })
	sched, _ = cron.ParseStandard("30 3-6,20-23 * * *")
	c.Schedule(sched, func() { fmt.Println(".. in the range 3-6am, 8-11pm") })
	sched, _ = cron.ParseStandard("CRON_TZ=Asia/Tokyo 30 04 * * *")
	c.Schedule(sched, func() { fmt.Println("Runs at 04:30 Tokyo time every day") })
	sched, _ = cron.ParseStandard("@hourly")
	c.Schedule(sched, func() { fmt.Println("Every hour, starting an hour from now") })
	sched, _ = cron.ParseStandard("@every 1h30m")
	c.Schedule(sched, func() { fmt.Println("Every hour thirty, starting an hour thirty from now") })

	c.Start()
	..
	// Funcs are invoked in their own goroutine, asynchronously.
	...
	// Funcs may also be added to a running Cron
	sched, _ = cron.ParseStandard("@daily")
	c.Schedule(sched, func() { fmt.Println("Every day") })
	..
	// Inspect the cron job entries' next and previous run times.
	inspect(c.Entries())
	..
	c.Stop()  // Stop the scheduler (does not stop any jobs already running).

# CRON Expression Format

A cron expression represents a set of times, using 5 space-separated fields.

	Field name   | Mandatory? | Allowed values  | Allowed special characters
	----------   | ---------- | --------------  | --------------------------
	Minutes      | Yes        | 0-59            | * / , -
	Hours        | Yes        | 0-23            | * / , -
	Day of month | Yes        | 1-31            | * / , - ? L W
	Month        | Yes        | 1-12 or JAN-DEC | * / , -
	Day of week  | Yes        | 0-6 or SUN-SAT  | * / , - ? L #

Month and Day-of-week field values are case insensitive.  "SUN", "Sun", and
"sun" are equally accepted.

The specific interpretation of the format is mainly based on the Cron Wikipedia page:
https://en.wikipedia.org/wiki/Cron

# Alternative Formats

Alternative Cron expression formats support other fields like seconds. This
can be implemented by creating a custom Parser to generate a Schedule:

	parser, _ := cron.NewDefaultParser(
		cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	sched, _ := parser.Parse("* * * * * *")

To emulate Quartz, the most popular alternative Cron schedule format, the
seconds field can be required:
http://www.quartz-scheduler.org/documentation/quartz-2.x/tutorials/crontrigger.html

	parser, _ := cron.NewDefaultParser(
		cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

# Special Characters

Asterisk ( * )

The asterisk indicates that the cron expression will match for all values of the
field; e.g., using an asterisk in the 5th field (month) would indicate every
month.

Slash ( / )

Slashes are used to describe increments of ranges. For example 3-59/15 in the
1st field (minutes) would indicate the 3rd minute of the hour and every 15
minutes thereafter. The form "*\/..." is equivalent to the form "first-last/...",
that is, an increment over the largest possible range of the field.  The form
"N/..." is accepted as meaning "N-MAX/...", that is, starting at N, use the
increment until the end of that specific range.  It does not wrap around.

Comma ( , )

Commas are used to separate items of a list. For example, using "MON,WED,FRI" in
the 5th field (day of week) would mean Mondays, Wednesdays and Fridays.

Hyphen ( - )

Hyphens are used to define ranges. For example, 9-17 would indicate every
hour between 9am and 5pm inclusive.

Question mark ( ? )

Question mark may be used instead of '*' for leaving either day-of-month or
day-of-week blank.

Last ( L )

L char can be used in day-of-month field meaning the last day of the month.
It can be used subtracting a number of days, for example L-1 would mean
"One day before the last day of the month" (30th for months with 31 days).
Cannot be used with steps (slash).

L char can be also used in day-of-week field. If alone, it's equivalent to 6 (Saturday).
If used after a day-of-week value it indicates the last occurence of that dow in the month.
For example, FRIL would mean the last friday of the month.
Cannot be used with steps or ranges.

L char can be combined with W (LW) meaning the last weekday of the month.
LW cannot be used with steps or ranges.

Nearest weekday ( W )

W char can be used in day-of-month field to specify the weekday (Monday-Friday) nearest the given day.
For example, given 19W dom, if the 19th is Saturday the expression would activate on Friday 18th.
If the 19th is a Sunday instead, the expression would activate on Monday 20th.
The W char looks for weekdays within the month boundaries. Given a 1W dom with the 1st being a Saturday,
the expression would activate on Monday 3rd.
Cannot be used with steps or ranges.

Nth occurrence ( # )

Nth occurence char can be used in day-of-week field and allows to specify an occurence of a specific dow within the month.
For example, SUN#2 means the second Sunday of the month.
Cannot be used with steps or ranges.

# Predefined schedules

One of several pre-defined schedules may be used in place of a cron expression.

	Entry                  | Description                                | Equivalent To
	-----                  | -----------                                | -------------
	@yearly (or @annually) | Run once a year, midnight, Jan. 1st        | 0 0 1 1 *
	@monthly               | Run once a month, midnight, first of month | 0 0 1 * *
	@weekly                | Run once a week, midnight between Sat/Sun  | 0 0 * * 0
	@daily (or @midnight)  | Run once a day, midnight                   | 0 0 * * *
	@hourly                | Run once an hour, beginning of hour        | 0 * * * *

# Intervals

Jobs may also be scheduled to execute at fixed intervals, starting at the time they are added
or cron is run. This is supported by formatting the cron spec like this:

	@every <duration>

where "duration" is a string accepted by time.ParseDuration
(http://golang.org/pkg/time/#ParseDuration), multiple of one second.

For example, "@every 1h30m10s" would indicate a schedule that activates after
1 hour, 30 minutes, 10 seconds, and then every interval after that.

Note: The interval does not take the job runtime into account.  For example,
if a job takes 3 minutes to run, and it is scheduled to run every 5 minutes,
it will have only 2 minutes of idle time between each run.

# Clock and time zones

Cron use a [Clock] interface when interacting with time. A custom clock can be set using
[WithClock] option. [NewTimerSkippingInstantExecutionClock] can be helpful for testing purposes.

The clock is also responsible for defining the timezone to be used when applying the cron schedule.
The default clock can be created with a different timezone using [NewDefaultClock].
By default, all interpretation and scheduling is done in the machine's local
time zone (time.Local).

Individual cron schedules may also override the time zone they are to be
interpreted in by providing an additional space-separated field at the beginning
of the cron spec, of the form "CRON_TZ=Asia/Tokyo".

For example:

	# Runs at 6am in time.Local
	sched, _ := cron.ParseStandard("0 6 * * ?")
	cron.New().Schedule(sched, ...)

	# Runs at 6am in America/New_York
	nyc, _ := time.LoadLocation("America/New_York")
	c := cron.New(cron.WithClock(cron.NewDefaultClock(nyc, cron.DefaultNopTimer)))
	sched, _ = cron.ParseStandard("0 6 * * ?")
	c.Schedule(sched, ...)

	# Runs at 6am in Asia/Tokyo
	sched, _ = cron.ParseStandard("CRON_TZ=Asia/Tokyo 0 6 * * ?")
	cron.New().Schedule(sched, ...)

The prefix "TZ=(TIME ZONE)" is also supported for legacy compatibility.

Be aware that jobs scheduled during daylight-savings leap-ahead transitions will
not be run!

# Job Wrappers

A Cron runner may be configured with a chain of job wrappers to add
cross-cutting functionality to all submitted jobs. For example, they may be used
to achieve the following effects:

  - Delay a job's execution if the previous run hasn't completed yet
  - Skip a job's execution if the previous run hasn't completed yet
  - Log each job's invocations

Install wrappers for all jobs added to a cron using the `cron.WithChain` option:

	cron.New(cron.WithChain(
		cron.SkipIfStillRunning(logger),
	))

Install wrappers for individual jobs by explicitly wrapping them:

	job = cron.NewChain(
		cron.SkipIfStillRunning(logger),
	).Then(job)

# Thread safety

Since the Cron service runs concurrently with the calling code, some amount of
care must be taken to ensure proper synchronization.

All cron methods are designed to be correctly synchronized as long as the caller
ensures that invocations have a clear happens-before ordering between them.

# Logging

Cron use [log/slog] package for logging. The logger can be set through its option:

	cron.New(cron.WithLogger(slog.Default()))

# Implementation

Cron entries are stored in a min heap based on their next activation time.  Cron
sleeps until the next job is due to be run.

Upon waking:
  - it runs each entry that is active on that second
  - it calculates the next run times for the jobs that were run
  - it updates the heap of entries by next activation time.
  - it goes to sleep until the soonest job.

[log/slog]: https://pkg.go.dev/log/slog
*/
package cron
