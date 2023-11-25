#!/usr/bin/env bash

set -Eeuo pipefail

PORT=8080 go run main.go bot.go db.go fetchers.go