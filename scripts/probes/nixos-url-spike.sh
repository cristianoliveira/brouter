#!/bin/sh
# TASK-0007 spike tooling: URL-delivery probe for NixOS + Sway (Wayland).
# Run INSIDE the graphical Sway session on the target NixOS machine.
#
# Simplified per product direction: read-only inspection is separate from
# the explicit opt-in probe, and the probe NEVER mutates system state —
# registrations live in an isolated XDG environment inside the spike
# workspace, so there is nothing to restore and no default can change.
#
# Commands:
#   inspect     read-only: session facts, browser resolution, portals
#   install     write the isolated logging handler into the workspace
#   probe N     hand fixed benign URL probe-N to the OS opener (N = 1|2)
#   reset       remove workspace registrations (safe when absent/empty)
#   selftest    host-agnostic checks (no Linux/session requirement)
#   help        this text
#
# Gates: every command except reset/selftest requires NixOS
# (os-release ID=nixos), WAYLAND_DISPLAY, and a validated Sway session
# (swaymsg -t get_version). URLs are fixed and benign by construction;
# the receipt logger additionally redacts query/fragment/userinfo and
# never logs raw argv.
set -eu

SPIKE_DIR="${BROUTER_SPIKE_DIR:-$PWD/brouter-spike}"
HANDLER_ID="brouter-spike-handler"

log() { printf '%s\n' "$*"; }
die() { printf '%s\n' "refusing: $*" >&2; exit 1; }

require_nixos_sway() {
	# Selftest-only hook: lets the controlled harness exercise install and
	# probe paths on any host. Real users never set this; the session gate
	# stays closed without it.
	if [ "${BROUTER_SPIKE_SKIP_SESSION_GATE:-0}" = "1" ]; then
		return 0
	fi
	[ "$(uname -s)" = "Linux" ] || die "this is not Linux; the spike must run on the NixOS target"
	grep -q '^ID=nixos' /etc/os-release 2>/dev/null || die "host is not NixOS (os-release lacks ID=nixos)"
	[ -n "${WAYLAND_DISPLAY:-}" ] || die "WAYLAND_DISPLAY is not set; a real Sway Wayland session is required"
	command -v swaymsg >/dev/null 2>&1 || die "swaymsg is required to validate the Sway session"
	swaymsg -t get_version >/dev/null 2>&1 ||
		die "swaymsg validation failed; a real Sway session is required (not X11, not another compositor)"
	command -v xdg-open >/dev/null 2>&1 || die "xdg-open is required and missing"
	command -v xdg-mime >/dev/null 2>&1 || die "xdg-mime is required and missing"
}

# isolated_env redirects all XDG state into the spike workspace: handler
# registrations land here and the system default handler is never touched.
isolated_env() {
	export XDG_DATA_HOME="$SPIKE_DIR/xdg-data"
	export XDG_CONFIG_HOME="$SPIKE_DIR/xdg-config"
	mkdir -p "$XDG_DATA_HOME/applications" "$XDG_CONFIG_HOME"
}

# probe_url maps a fixed index to a benign, secret-free URL. No
# user-supplied URLs are accepted, so nothing sensitive can leak in.
probe_url() {
	case $1 in
	1) printf 'https://spike.example/probe-1' ;;
	2) printf 'https://spike.example/probe-2' ;;
	*) return 1 ;;
	esac
}

# write_receiver emits the receipt logger. It derives its log file from
# its own location, redacts query/fragment/userinfo, and never logs raw
# argv — only the argument count.
write_receiver() {
	cat > "$1" <<'RECEIVER'
#!/bin/sh
set -eu
redact() {
	url=$1
	case "$url" in *@*) printf 'REDACTED (userinfo credentials in url)'; return ;; esac
	base=${url%%\?*}
	base=${base%%\#*}
	printf '%s (query/fragment redacted; %s bytes, cksum %s)' "$base" "${#url}" \
		"$(printf '%s' "$url" | cksum 2>/dev/null | cut -d' ' -f1)"
}
log_file=$(dirname "$0")/received.log
{
	printf 'date=%s\n' "$(date '+%Y-%m-%dT%H:%M:%S%z' 2>/dev/null || date)"
	printf 'url: '
	redact "$1"
	printf '\n'
	printf 'argcount=%s\n' "$#"
	printf 'WAYLAND_DISPLAY=%s XDG_SESSION_TYPE=%s XDG_CURRENT_DESKTOP=%s\n' \
		"${WAYLAND_DISPLAY:-}" "${XDG_SESSION_TYPE:-}" "${XDG_CURRENT_DESKTOP:-}"
	printf 'invoked-by=%s\n' "$(ps -o comm= -p $PPID 2>/dev/null || echo unknown)"
} >> "$log_file"
RECEIVER
	chmod +x "$1"
}

cmd_inspect() {
	require_nixos_sway
	echo "== session =="
	echo "uname: $(uname -s -m)"
	echo "os-release: $(grep -E '^(ID|VARIANT_ID)=' /etc/os-release 2>/dev/null | tr '\n' ' ')"
	echo "session: XDG_SESSION_TYPE=${XDG_SESSION_TYPE:-unset} XDG_CURRENT_DESKTOP=${XDG_CURRENT_DESKTOP:-unset} WAYLAND_DISPLAY=${WAYLAND_DISPLAY:-unset} SWAYSOCK=${SWAYSOCK:+set}"
	echo "== browser executable resolution (graphical session PATH) =="
	for browser in brave brave-browser google-chrome-stable chromium firefox; do
		printf '%s -> %s\n' "$browser" "$(command -v "$browser" || echo not-found)"
	done
	echo "== Nix system browser desktop Exec lines =="
	for dir in /run/current-system/sw/share/applications /etc/profiles/per-user/*/share/applications /nix/var/nix/profiles/profile/share/applications "$HOME/.local/share/applications" /usr/share/applications; do
		for entry in "$dir"/*.desktop; do
			[ -f "$entry" ] || continue
			if grep -qiE '^(Name|Exec)=.*(brave|chrome|chromium|firefox)' "$entry" 2>/dev/null; then
				echo "--- $entry"
				grep -E '^Exec=' "$entry"
			fi
		done
	done
	echo "== portals =="
	if command -v systemctl >/dev/null 2>&1; then
		systemctl --user list-units "xdg-desktop-portal*" --no-pager 2>/dev/null | head -10
	fi
}

cmd_install() {
	require_nixos_sway
	isolated_env
	write_receiver "$SPIKE_DIR/receive.sh"

	handler="$SPIKE_DIR/xdg-data/applications/$HANDLER_ID.desktop"
	sed "s|@SPIKE_DIR@|$SPIKE_DIR|g" > "$handler" <<DESKTOP
[Desktop Entry]
Type=Application
Name=brouter spike handler
Exec=@SPIKE_DIR@/receive.sh %u
Terminal=false
NoDisplay=true
MimeType=x-scheme-handler/http;x-scheme-handler/https;
DESKTOP

	command -v desktop-file-validate >/dev/null 2>&1 && desktop-file-validate "$handler"
	update-desktop-database "$SPIKE_DIR/xdg-data/applications" 2>/dev/null || true

	xdg-mime default "$HANDLER_ID.desktop" x-scheme-handler/http
	xdg-mime default "$HANDLER_ID.desktop" x-scheme-handler/https

	log "installed: isolated handler registered (system defaults untouched)"
	log "next: probe 1, probe 2; receipt: $SPIKE_DIR/received.log"
}

cmd_probe() {
	require_nixos_sway
	isolated_env
	handler="$SPIKE_DIR/xdg-data/applications/$HANDLER_ID.desktop"
	[ -f "$handler" ] || die "isolated handler not installed yet; run install first"

	index=${1:-}
	url=$(probe_url "$index") || die "unknown probe index '$index' (use 1 or 2)"

	set +e
	xdg-open "$url"
	opener_rc=$?
	set -e
	if [ "$opener_rc" -eq 0 ]; then
		log "opener accepted $url (exit 0) — receipt, not proof: check $SPIKE_DIR/received.log"
		exit 0
	fi
	log "opener FAILED (exit $opener_rc) — failure is visible by design; check received.log"
	exit "$opener_rc"
}

cmd_reset() {
	# Session-less by design: recovery must work even if the Sway session
	# died mid-spike. Absent and empty workspaces are both safe.
	if [ -d "$SPIKE_DIR/xdg-data" ] || [ -d "$SPIKE_DIR/xdg-config" ]; then
		rm -rf "$SPIKE_DIR/xdg-data" "$SPIKE_DIR/xdg-config"
		log "reset: removed workspace registrations"
	else
		log "reset: nothing to remove"
	fi
}

cmd_help() {
	log "usage: nixos-url-spike.sh {inspect|install|probe N|reset|selftest|help}"
	log "Run inside the NixOS Sway session. Registrations are isolated to"
	log "$SPIKE_DIR and the system default handler is never modified."
}

cmd_selftest() {
	# Host-agnostic controlled harness. Session-gate refusal is forced by
	# removing WAYLAND_DISPLAY; install/probe paths run with the documented
	# selftest hook and fake xdg tools. set +e: failures are
	# checked explicitly, never by the shell exiting.
	set +e
	run() { "$0" "$@"; }
	work=$(mktemp -d)
	fakes=$work/bin
	mkdir -p "$fakes"

	printf '#!/bin/sh\nprintf "%%s\\n" "$*" >> %s/xdg-mime.calls\nexit 0\n' "$work" > "$fakes/xdg-mime"
	printf '#!/bin/sh\nprintf "%%s\\n" "$*" >> %s/xdg-open.calls\nrc=${XDG_FAKE_OPEN_RC:-0}\n[ "$rc" -ne 0 ] || "$BROUTER_SPIKE_DIR/receive.sh" "$1"\nexit "$rc"\n' "$work" > "$fakes/xdg-open"
	chmod +x "$fakes/xdg-mime" "$fakes/xdg-open"

	fail=0
	check() {
		condition=$1
		description=$2
		if [ "$condition" = 0 ]; then
			log "selftest ok: $description"
		else
			log "selftest FAIL: $description"
			fail=1
		fi
	}

	# 1. session gate: install without a Sway session refuses.
	out=$(env -u BROUTER_SPIKE_SKIP_SESSION_GATE -u WAYLAND_DISPLAY PATH="$fakes:$PATH" "$0" install 2>&1)
	gate_rc=$?
	check $(test "$gate_rc" -ne 0; echo $?) "install without a Sway session refuses"
	check $(printf '%s' "$out" | grep -q "Sway Wayland session"; echo $?) "gate refusal names the Sway requirement"

	# 2-6. functional paths with the documented selftest hook.
	export PATH="$fakes:$PATH"
	export BROUTER_SPIKE_SKIP_SESSION_GATE=1
	export BROUTER_SPIKE_DIR="$work/spike"
	export HOME="$work/home"
	mkdir -p "$HOME"

	out=$(run install)
	check "$?" "install succeeds in isolated mode"
	check $(test -f "$BROUTER_SPIKE_DIR/receive.sh"; echo $?) "receiver written into workspace"
	check $(test -f "$BROUTER_SPIKE_DIR/xdg-data/applications/$HANDLER_ID.desktop"; echo $?) "handler desktop file isolated"
	check $(grep -c "default $HANDLER_ID.desktop" "$work/xdg-mime.calls" | grep -q 2; echo $?) "http and https registered"
	check $(test ! -e "$HOME/.local/share/applications/$HANDLER_ID.desktop"; echo $?) "real applications dir untouched"

	out=$(run probe 1)
	check "$?" "probe 1 succeeds with passing opener"
	check $(grep -q "probe-1" "$BROUTER_SPIKE_DIR/received.log"; echo $?) "receipt recorded probe-1"

	out=$(run probe 2)
	check "$?" "probe 2 succeeds (sequential delivery)"
	check $(grep -q "probe-2" "$BROUTER_SPIKE_DIR/received.log"; echo $?) "receipt recorded probe-2"

	export XDG_FAKE_OPEN_RC=9
	run probe 1
	probe_rc=$?
	unset XDG_FAKE_OPEN_RC
	check $(test "$probe_rc" = 9; echo $?) "probe propagates opener failure exit 9"

	out=$(run reset)
	reset_rc=$?
	check "$reset_rc" "reset succeeds"
	check $(test ! -d "$BROUTER_SPIKE_DIR/xdg-data"; echo $?) "reset removed isolated registrations"

	out=$(run reset)
	check "$?" "reset is safe when already absent"

	unset BROUTER_SPIKE_SKIP_SESSION_GATE

	# 7. receiver redaction: secrets never reach the receipt.
	dir=$(mktemp -d)
	write_receiver "$dir/receive.sh"
	"$dir/receive.sh" "https://spike.example/?token=super-secret-123"
	check $(if grep -q "super-secret-123" "$dir/received.log"; then false; else true; fi; echo $?) "receipt redacts url secrets"
	check $(grep -q "query/fragment redacted" "$dir/received.log"; echo $?) "receipt marks redaction"
	rm -rf "$dir"

	rm -rf "$work"
	if [ "$fail" = 0 ]; then
		log "selftest PASS: session gate, isolated install, probes, failure propagation, reset, and redaction verified"
	fi
	exit "$fail"
}

case ${1:-} in
inspect)
	require_nixos_sway
	cmd_inspect
	;;
install)
	require_nixos_sway
	cmd_install
	;;
probe)
	require_nixos_sway
	shift
	cmd_probe "$@"
	;;
reset)
	cmd_reset
	;;
selftest)
	cmd_selftest
	;;
help | --help | -h)
	cmd_help
	;;
*)
	cmd_help >&2
	exit 2
	;;
esac
