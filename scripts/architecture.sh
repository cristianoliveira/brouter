#!/bin/sh
# Architecture boundary enforcement from Go package metadata
# (docs/ARCHITECTURE.md rule 1): internal/domain imports the standard
# library only — no cmd, no infra, no third-party packages.
set -u

dir=${1:-.}
rc=0

stdlib=$(go -C "$dir" list std) || exit 1

domain_packages=$(go -C "$dir" list ./internal/domain/... 2>/dev/null)
if [ -z "$domain_packages" ]; then
	echo "architecture: no internal/domain packages; boundary trivially holds"
	exit 0
fi

for pkg in $domain_packages; do
	imports=$(go -C "$dir" list -f '{{range .Imports}}{{println .}}{{end}}' "$pkg")
	for imp in $imports; do
		case "
$stdlib
" in
		*"
$imp
"*) ;;
		*)
			echo "boundary violation: $pkg imports $imp (domain must use the standard library only)"
			rc=1
			;;
		esac
	done
done

if [ "$rc" -eq 0 ]; then
	echo "architecture: domain boundaries hold"
fi
exit $rc
