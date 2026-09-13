#!/bin/sh
# go vet over the module. Separate from the normal gate (TASK-0005).
set -u

dir=${1:-.}
exec go -C "$dir" vet ./...
