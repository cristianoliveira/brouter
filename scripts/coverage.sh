#!/bin/sh
# Per-package coverage against a budget file. The budget is the recorded
# floor: lowering a number is an explained regression and must state its
# rationale in the commit message that changes it.
# Usage: coverage.sh [dir] [budget-file]
# Budget lines: <package-import-path> <minimum-percent>
set -u

dir=${1:-.}
budget=${2:-scripts/coverage.budget}

results=$(go -C "$dir" test -cover ./...) || {
	echo "coverage: tests failed; fix them before measuring coverage"
	printf '%s\n' "$results"
	exit 1
}
printf '%s\n' "$results"

rc=0
while read -r pkg minimum; do
	case "$pkg" in ""|\#*) continue ;; esac
	actual=$(printf '%s\n' "$results" | grep -E "^ok[[:space:]]+$pkg([[:space:]]|$)" | sed -E 's/.*coverage: ([0-9.]+)%.*/\1/')
	if [ -z "$actual" ]; then
		echo "coverage: no coverage reported for budgeted package $pkg (missing tests?)"
		rc=1
		continue
	fi
	below=$(awk -v a="$actual" -v m="$minimum" 'BEGIN { print (a < m) ? 1 : 0 }')
	if [ "$below" = "1" ]; then
		echo "coverage: $pkg $actual% is below budget $minimum%"
		rc=1
	else
		echo "coverage: $pkg $actual% meets budget $minimum%"
	fi
done <"$budget"

exit $rc
