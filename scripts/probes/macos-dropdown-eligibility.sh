#!/bin/sh
# Read-only macOS browser-dropdown eligibility probe for BrouterHandler.
#
# Reports the LaunchServices facts that decide whether the app can
# appear in System Settings > Desktop & Dock > Default web browser:
# claimed schemes, claimed document types (the browser-category marker
# UTI plus html/xhtml), role, and whether every registration points at
# a bundle that still exists on disk.
#
# This probe NEVER mutates state: no lsregister -f/-u, no defaults
# writes, no process kills, no LaunchServices database reset.
set -u

LSREG="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
APP_ID="com.cristianoliveira.brouter.handler"

dump=$("$LSREG" -dump 2>/dev/null)

# Sections run from a bundle-id line to the next separator; path lines
# precede identifier lines, so the whole section must be buffered
# before deciding it is ours.
section=$(printf '%s\n' "$dump" | awk -v id="$APP_ID" '
	/^-+$/ {
		if (is_ours) out = out buf
		buf = ""
		is_ours = 0
		next
	}
	{
		buf = buf $0 "\n"
		if (index($0, "identifier:") && index($0, id)) is_ours = 1
	}
	END {if (is_ours) out = out buf; printf "%s", out}
')

report() { printf '%s\n' "$*"; }

if [ -z "$section" ]; then
	report "result: NOT REGISTERED — no LaunchServices entry for $APP_ID"
	report "fix: install the bundle (see docs/macos-handler.md) and register it with lsregister -f"
	exit 1
fi

paths=$(printf '%s\n' "$section" | grep '^path:' | sed 's/^path: *//' | sed 's/ (0x[0-9a-f]*)$//' | sort -u)
live=""
dead=""
for p in $paths; do
	if [ -d "$p" ]; then
		live="$live $p"
	else
		dead="$dead $p"
	fi
done

schemes=$(printf '%s\n' "$section" | grep 'claimed schemes:' | sed 's/claimed schemes: *//' | sort -u | tr '\n' ' ')
utis=$(printf '%s\n' "$section" | grep 'claimed UTIs:' | sed 's/claimed UTIs: *//' | sort -u | tr '\n' ' ')
role=$(printf '%s\n' "$section" | grep -m1 'roles:')

report "registered paths: $paths"
[ -n "$dead" ] && report "warning: dangling registrations (bundle gone): $dead"
report "live paths:$live"
report "claimed schemes: $schemes"
report "claimed UTIs: $utis"
report "$role"

missing=""
for marker in "com.apple.default-app.web-browser" "public.html" "public.xhtml"; do
	printf '%s\n' "$utis" | grep -q "$marker" || missing="$missing $marker"
done
if [ -n "$missing" ]; then
	report "result: NOT ELIGIBLE — missing document-type claims:$missing"
	report "fix: rebuild with the browser-category CFBundleDocumentTypes (see docs/macos-handler.md), reinstall, re-register"
	exit 1
fi
[ -z "$live" ] && report "result: NOT ELIGIBLE — no registration points at an existing bundle"
[ -n "$live" ] || exit 1

printf '%s\n' "$section" | grep -q 'claimed schemes:.*http:' || {
	report "result: NOT ELIGIBLE — http scheme not claimed"
	exit 1
}
report "result: metadata-eligible (claims browser category + html/xhtml + http scheme at a live path)"
report "note: final dropdown membership is System Settings UI behavior; confirm visually and record as user-attested evidence"
