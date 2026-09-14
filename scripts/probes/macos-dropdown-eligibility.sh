#!/bin/sh
# Read-only macOS browser-dropdown eligibility probe for BrouterHandler.
#
# Reports the LaunchServices facts that decide whether the app can
# appear in System Settings > Desktop & Dock > Default web browser.
# Every registration section for the bundle identifier is evaluated
# independently: eligibility requires ONE live path whose own claims
# include the http and https schemes AND the html/xhtml plus
# browser-category document types. Claims from dangling registrations
# never vouch for a live one.
#
# This probe NEVER mutates state: no lsregister -f/-u, no defaults
# writes, no process kills, no LaunchServices database reset.
set -u

LSREG="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
APP_ID="com.cristianoliveira.brouter.handler"

# Optional argument: path to a saved `lsregister -dump` output. The
# live database snapshot can vary between invocations while LS settles,
# so recording once and probing the recording keeps repeated analysis
# deterministic. Without an argument the probe captures its own dump.
dump_file=${1:-}
if [ -n "$dump_file" ]; then
	if [ ! -r "$dump_file" ]; then
		printf '%s\n' "refusing: dump file $dump_file is not readable" >&2
		exit 2
	fi
	dump=$(cat "$dump_file")
else
	dump=$("$LSREG" -dump 2>/dev/null)
fi

# One record per registration section:
# <path>\t<schemes>\t<claimed UTIs>
# Sections are buffered whole: path lines precede the identifier line
# inside each dump section, so ownership is only known after the
# section is complete.
records=$(printf '%s\n' "$dump" | awk -v id="$APP_ID" '
	function flush() {
		if (is_ours) {
			n = split(buf, L, "\n")
			for (i = 1; i <= n; i++) {
				line = L[i]
				if (index(line, "path:") == 1 && path == "") {
					path = line
					sub(/^path:[ \t]+/, "", path)
					sub(/ \(0x[0-9a-f]+\)$/, "", path)
				}
				if (index(line, "claimed schemes:") == 1) {
					v = line
					sub(/^claimed schemes:[ \t]*/, "", v)
					schemes = schemes " " v
				}
				if (index(line, "claimed UTIs:") == 1) {
					v = line
					sub(/^claimed UTIs:[ \t]*/, "", v)
					utis = utis " " v
				}
			}
			if (path != "") printf "%s\t%s\t%s\n", path, schemes, utis
		}
		is_ours = 0
		buf = ""
		path = ""
		schemes = ""
		utis = ""
	}
	/^-+$/ {flush(); next}
	{
		buf = buf $0 "\n"
		if (index($0, "identifier:") && index($0, id)) is_ours = 1
	}
	END {flush()}
')

report() { printf '%s\n' "$*"; }

if [ -z "$records" ]; then
	report "result: NOT REGISTERED — no LaunchServices entry for $APP_ID"
	report "fix: install the bundle (see docs/macos-handler.md) and register it with lsregister -f"
	exit 1
fi

found_live=0
result=""
while IFS="	" read -r path schemes utis; do
	[ -n "$path" ] || continue
	if [ ! -d "$path" ]; then
		report "warning: dangling registration (bundle gone): $path"
		continue
	fi
	found_live=1
	missing=""
	for marker in "http:" "https:"; do
		case $schemes in
			*"$marker"*) ;;
			*) missing="$missing $marker" ;;
		esac
	done
	for marker in "public.html" "public.xhtml" "com.apple.default-app.web-browser"; do
		case $utis in
			*"$marker"*) ;;
			*) missing="$missing $marker" ;;
		esac
	done
	if [ -z "$missing" ]; then
		result="eligible at $path"
		break
	fi
	report "note: live path $path is missing claims:$missing"
done <<EOF
$records
EOF

if [ "$found_live" -eq 0 ]; then
	report "result: NOT ELIGIBLE — no registration points at an existing bundle"
	exit 1
fi

if [ -n "$result" ]; then
	report "result: metadata-eligible ($result)"
	report "note: final dropdown membership is System Settings UI behavior; confirm visually and record as user-attested evidence"
	exit 0
fi
report "result: NOT ELIGIBLE — no live registration claims the full browser metadata set"
report "fix: rebuild with the browser-category CFBundleDocumentTypes (see docs/macos-handler.md), reinstall, re-register"
exit 1
