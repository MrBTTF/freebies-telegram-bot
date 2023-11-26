#!/usr/bin/env bash

set -Eeuo pipefail

GOOS=linux GOARCH=amd64 go build -v  \
    -ldflags "-linkmode 'external' -extldflags '-static'"  \
    -o build/app main.go bot.go db.go fetchers.go