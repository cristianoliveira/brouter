#!/bin/sh
# Race detector over all tests. Separate from the normal gate (TASK-0005).
set -u

dir=${1:-.}
exec go -C "$dir" test -race ./...
