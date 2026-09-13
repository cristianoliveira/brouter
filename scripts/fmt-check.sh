#!/bin/sh
# Non-mutating gofmt check: lists offending files and fails. Never writes;
# use fmt-fix.sh to rewrite.
set -u

dir=${1:-.}
unformatted=$(gofmt -l "$dir")
if [ -n "$unformatted" ]; then
	echo "$unformatted"
	echo "gofmt check failed; run 'sh scripts/fmt-fix.sh' to rewrite"
	exit 1
fi
