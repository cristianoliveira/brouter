#!/bin/sh
# Rewrites files to gofmt's canonical formatting. This is the only
# formatting command that mutates; the check stays non-mutating.
set -u

dir=${1:-.}
gofmt -w "$dir"
echo "formatted $dir"
