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

## Linting and Testing
Use the provided devcontainer as a reference for the required development tools.
See https://code.visualstudio.com/docs/devcontainers/containers for more details about vscode devcontainers.
```console
# run tests
go test -v -race ./...

# run linters
golangci-lint run .
```
