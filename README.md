# Cron

> By **G**oogle **D**evelopers **G**roup [Valle d'Aosta](https://gdg.community.dev/gdg-valle-daosta/)

<p align="center">
    <a href="https://goreportcard.com/badge/github.com/gdgvda/cron">
        <img src="https://goreportcard.com/badge/github.com/gdgvda/cron" alt="Go Report Card"/>
    </a>
    <a href="https://pkg.go.dev/github.com/gdgvda/cron">
        <img src="https://pkg.go.dev/badge/github.com/gdgvda/cron.svg" alt="Go Reference">
    </a>
    <a href="https://codecov.io/gh/gdgvda/cron"> 
        <img src="https://codecov.io/gh/gdgvda/cron/branch/master/graph/badge.svg"/ alt="Codecov badge"> 
    </a>
</p>

This repository is an independent fork of the https://github.com/robfig/cron project,
credits to all its contributors.

## Time Dependency and Clock Interface

This cron implementation supports unified time dependency through a `Clock` interface, enabling flexible time management for various scenarios including unit testing, time simulation, and custom time sources.

### Features

- **Clock Interface**: Abstraction for time operations with `Now()`, `Timer()`, and `SetTime()` methods
- **DefaultClock**: Production implementation using real system time
- **Time Travel**: Support for simulated time advancement in testing scenarios
- **Timer Abstraction**: `Timer` and `FireableTimer` interfaces for flexible timer management

### Use Cases

- **Unit Testing**: Mock time to verify business logic without waiting for real time
- **Time Simulation**: Start cron jobs from an earlier time or fast-forward through schedules
- **Custom Time Sources**: Use precise network time or other time providers
- **Testing Scenarios**: Step-by-step time advancement with `RunTo()` method

### Usage

```go
// Default usage with real time
c := cron.New()

// Custom clock for testing
fakeClock := NewFakeClock(time.UTC, time.Now())
c := cron.New(cron.WithClock(fakeClock))
c.Add("0 * * * *", func() {
    fmt.Printf("Job executed at: %v\n", fakeClock.Now())
})

// Time travel in tests
c.Start()
c.RunTo(time.Now().Add(time.Hour)) // Fast-forward 1 hour
```

## Linting and Testing
Use the provided devcontainer as a reference for the required development tools.
See https://code.visualstudio.com/docs/devcontainers/containers for more details about vscode devcontainers.
```console
# run tests
go test -v -race ./...

# run linters
golangci-lint run .
```
