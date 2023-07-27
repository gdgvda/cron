# Cron

> By **G**oogle **D**evelopers **G**roup [Valle d'Aosta](https://gdg.community.dev/gdg-valle-daosta/)

## Cron Linting and Testing with docker or podman

Podman and docker have the same api, so you can safely change podman to docker command if docker is used.

```console
$ podman build -t testenv .

$ podman run -it --rm -v $(pwd):/go/src -w /go/src testenv bash
```
then inside the pod you can run:

```console
:/go/src# go test

:/go/src# golangci-lint -v
```

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