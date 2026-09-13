#!/bin/sh
# TASK-0007 spike tooling: URL-delivery probe for NixOS + Sway (Wayland).
# Run INSIDE the graphical Sway session on the target NixOS machine.
#
# Two modes:
#   default            NON-MUTATING: every xdg registration happens in an
#                      isolated XDG environment inside the spike workspace.
#                      The system default handler is never touched, so
#                      there is nothing to restore.
#   --system           OPT-IN: mutates the real mimeapps.list to prove the
#                      actual default-handler path. Strict backup, an
#                      install-time safety trap (auto-restore on partial
#                      failure), and verified restore. Use only when
#                      isolated evidence is not sufficient.
#
# Usage:
#   nixos-url-spike.sh [--system] backup
#   nixos-url-spike.sh [--system] install
#   nixos-url-spike.sh [--system] probe URL
#   nixos-url-spike.sh [--system] browser DESKTOP-ID URL
#   nixos-url-spike.sh [--system] report
#   nixos-url-spike.sh [--system] restore      # session-less; works after a crash
#   nixos-url-spike.sh selftest                # host-agnostic redaction checks
#
# URL secrets are never logged: query strings, fragments, and userinfo are
# redacted; raw argv is not logged at all; credential-bearing URLs are
# rejected outright. DESKTOP-ID is the desktop file id without the
# .desktop suffix (e.g. brave-browser).
set -eu

SPIKE_DIR="${BROUTER_SPIKE_DIR:-$PWD/brouter-spike}"
HANDLER_ID="brouter-spike-handler"
APPLICATIONS_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
MIMEAPPS="${XDG_CONFIG_HOME:-$HOME/.config}/mimeapps.list"
DESKTOP_FILE="$APPLICATIONS_DIR/$HANDLER_ID.desktop"

log() { printf '%s\n' "$*"; }
die() { printf '%s\n' "refusing: $*" >&2; exit 1; }

require_nixos_sway() {
	[ "$(uname -s)" = "Linux" ] || die "this is not Linux; the spike must run on the NixOS target"
	grep -q '^ID=nixos' /etc/os-release 2>/dev/null || die "host is not NixOS (os-release lacks ID=nixos)"
	[ -n "${WAYLAND_DISPLAY:-}" ] || die "WAYLAND_DISPLAY is not set; a real Sway Wayland session is required"
	command -v swaymsg >/dev/null 2>&1 || die "swaymsg is required to validate the Sway session"
	swaymsg -t get_version >/dev/null 2>&1 ||
		die "swaymsg validation failed; a real Sway session is required (not X11, not another compositor)"
	command -v xdg-open >/dev/null 2>&1 || die "xdg-open is required and missing"
	command -v xdg-mime >/dev/null 2>&1 || die "xdg-mime is required and missing"
}

require_backup() {
	[ -f "$SPIKE_DIR/mimeapps.list.backup" ] || die "run 'backup' first; system-mode changes require a recorded restore point"
}

resolve_desktop_file() {
	id=${1%.desktop}
	for dir in \
		"${XDG_DATA_HOME:-$HOME/.local/share}/applications" \
		"$HOME/.local/share/applications" \
		/run/current-system/sw/share/applications \
		/etc/profiles/per-user/*/share/applications \
		/nix/var/nix/profiles/profile/share/applications \
		/usr/local/share/applications \
		/usr/share/applications; do
		[ -f "$dir/$id.desktop" ] && {
			echo "$dir/$id.desktop"
			return 0
		}
	done
	return 1
}

# redact_url keeps scheme, host, and path for correlation and strips
# query, fragment, and userinfo, which commonly carry secrets.
redact_url() {
	url=$1
	case "$url" in
	*@*) printf 'REDACTED (url contained userinfo credentials)'; return ;;
	esac
	base=${url%%\?*}
	base=${base%%\#*}
	printf '%s (query/fragment redacted; full url %s bytes, cksum %s)' \
		"$base" "${#url}" "$(printf '%s' "$url" | cksum 2>/dev/null | cut -d' ' -f1)"
}

reject_secret_urls() {
	case $1 in
	*@*) die "url contains userinfo credentials (user:pass@host); remove them before probing" ;;
	esac
}

# write_receiver emits the receipt logger. It derives its log file from
# its own location, so the same code works in the spike workspace and in
# the selftest sandbox. Raw argv is never written.
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

# install_safety_trap: in --system mode, a partially failed install must
# not leave a half-registered default handler behind. Cleared on success.
install_safety_trap() {
	trap 'restore_state >/dev/null 2>&1 || true' EXIT
}

clear_install_trap() {
	trap - EXIT
}

cmd_backup() {
	require_nixos_sway
	mkdir -p "$SPIKE_DIR"
	rm -f "$SPIKE_DIR/mimeapps.list.absent"

	if [ "$SYSTEM_MODE" = 1 ]; then
		if [ -f "$MIMEAPPS" ]; then
			cp "$MIMEAPPS" "$SPIKE_DIR/mimeapps.list.backup"
		else
			: > "$SPIKE_DIR/mimeapps.list.absent"
			: > "$SPIKE_DIR/mimeapps.list.backup"
			log "note: no mimeapps.list exists; system-mode restore will remove the generated file"
		fi
		: > "$SPIKE_DIR/handler-state.backup"
		for scheme in x-scheme-handler/http x-scheme-handler/https; do
			current=$(xdg-mime query default "$scheme" 2>/dev/null || echo none)
			printf '%s %s\n' "$scheme" "$current" >> "$SPIKE_DIR/handler-state.backup"
			log "backup: $scheme handled by: $current"
		done
	else
		log "isolated mode: system defaults are never touched; no backup needed"
	fi
	log "backup complete in $SPIKE_DIR"
}

cmd_install() {
	require_nixos_sway
	if [ "$SYSTEM_MODE" = 1 ]; then
		require_backup
		install_safety_trap
	else
		isolated_env
	fi
	mkdir -p "$SPIKE_DIR"
	write_receiver "$SPIKE_DIR/receive.sh"

	if [ "$SYSTEM_MODE" = 1 ]; then
		if [ -e "$DESKTOP_FILE" ]; then
			cp "$DESKTOP_FILE" "$SPIKE_DIR/desktop.preexisting.backup"
			die "existing $DESKTOP_FILE was backed up to $SPIKE_DIR/desktop.preexisting.backup; resolve it, then rerun install"
		fi
		dst=$DESKTOP_FILE
	else
		dst="$XDG_DATA_HOME/applications/$HANDLER_ID.desktop"
	fi

	sed "s|@SPIKE_DIR@|$SPIKE_DIR|g" > "$dst" <<DESKTOP
[Desktop Entry]
Type=Application
Name=brouter spike handler
Exec=@SPIKE_DIR@/receive.sh %u
Terminal=false
NoDisplay=true
MimeType=x-scheme-handler/http;x-scheme-handler/https;
DESKTOP

	command -v desktop-file-validate >/dev/null 2>&1 && desktop-file-validate "$dst"
	update-desktop-database "$(dirname "$dst")" 2>/dev/null || true

	xdg-mime default "$HANDLER_ID.desktop" x-scheme-handler/http
	xdg-mime default "$HANDLER_ID.desktop" x-scheme-handler/https

	if [ "$SYSTEM_MODE" = 1 ]; then
		clear_install_trap
		log "system mode: real default handler registered (safety trap cleared on success)"
		log "uninstall anytime with: $0 --system restore"
	else
		log "isolated mode: registrations live only in $SPIKE_DIR/xdg-*; system defaults untouched"
	fi
	log "next: run 'probe URL' (terminal) and/or click a link in another app; receipt: $SPIKE_DIR/received.log"
}

run_opener() {
	xdg-open "$1"
}

cmd_probe() {
	require_nixos_sway
	[ "$SYSTEM_MODE" = 1 ] && require_backup
	if [ "$SYSTEM_MODE" = 0 ]; then
		isolated_env
		[ -f "$XDG_DATA_HOME/applications/$HANDLER_ID.desktop" ] ||
			die "isolated handler not installed yet; run install first"
	fi
	url=${1:-}
	case "$url" in
	https://* | http://*) reject_secret_urls "$url" ;;
	*)
		log "usage: $0 probe https://spike.example/path" >&2
		exit 2
		;;
	esac

	run_opener "$url"
	opener_rc=$?
	if [ "$opener_rc" -eq 0 ]; then
		log "opener accepted the URL (exit 0) — receipt, not proof: check received.log"
		exit 0
	fi
	log "opener FAILED (exit $opener_rc) — failure is visible by design; check received.log and portal logs"
	exit "$opener_rc"
}

cmd_browser() {
	require_nixos_sway
	[ "$SYSTEM_MODE" = 1 ] && require_backup
	if [ "$SYSTEM_MODE" = 0 ]; then
		isolated_env
	fi
	desktop=${1:-}
	url=${2:-}
	case "$url" in
	https://* | http://*) reject_secret_urls "$url" ;;
	*)
		log "usage: $0 browser DESKTOP-ID URL" >&2
		exit 2
		;;
	esac

	desktop=${desktop%.desktop}
	desktop_file=$(resolve_desktop_file "$desktop") ||
		die "no desktop file found for id '$desktop' in user or system application directories"
	log "desktop file: $desktop_file"
	grep -E '^Exec=' "$desktop_file" || die "desktop file has no Exec line"
	exec_line=$(grep -E '^Exec=' "$desktop_file" | head -n 1 | cut -d= -f2-)
	exec_binary=${exec_line%% *}
	command -v "$exec_binary" >/dev/null 2>&1 ||
		log "warning: Exec binary '$exec_binary' not on PATH; Nix desktop Exec lines often reference /nix/store paths directly"

	xdg-mime default "$desktop" x-scheme-handler/http
	xdg-mime default "$desktop" x-scheme-handler/https

	run_opener "$url"
	opener_rc=$?
	if [ "$opener_rc" -eq 0 ]; then
		log "handed to $desktop — record whether the browser was COLD or already RUNNING, then repeat for the other state"
		exit 0
	fi
	log "opener FAILED (exit $opener_rc) — visible by design"
	exit "$opener_rc"
}

cmd_report() {
	require_nixos_sway
	mkdir -p "$SPIKE_DIR"
	{
		echo "== session =="
		echo "uname: $(uname -s -m)"
		echo "os-release: $(grep -E '^(ID|VARIANT_ID)=' /etc/os-release 2>/dev/null | tr '\n' ' ')"
		echo "session: XDG_SESSION_TYPE=${XDG_SESSION_TYPE:-unset} XDG_CURRENT_DESKTOP=${XDG_CURRENT_DESKTOP:-unset} WAYLAND_DISPLAY=${WAYLAND_DISPLAY:-unset} SWAYSOCK=${SWAYSOCK:+set}"
		echo "== Nix system browser desktop Exec resolution =="
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
		echo "== mode =="
		echo "SYSTEM_MODE=$SYSTEM_MODE (isolated registrations under $SPIKE_DIR/xdg-* unless --system)"
	} > "$SPIKE_DIR/report.txt"
	cat "$SPIKE_DIR/report.txt"
	log "report written: $SPIKE_DIR/report.txt"
}

restore_state() {
	if [ "$SYSTEM_MODE" != 1 ]; then
		log "isolated mode: nothing to restore — system defaults were never touched"
		return 0
	fi

	if [ -f "$SPIKE_DIR/desktop.preexisting.backup" ]; then
		cp "$SPIKE_DIR/desktop.preexisting.backup" "$DESKTOP_FILE"
		log "restored: pre-existing $HANDLER_ID.desktop"
	elif [ -f "$DESKTOP_FILE" ] && [ -f "$SPIKE_DIR/receive.sh" ]; then
		rm -f "$DESKTOP_FILE"
		log "removed: spike-created $HANDLER_ID.desktop"
	fi
	update-desktop-database "$APPLICATIONS_DIR" 2>/dev/null || true

	if [ -f "$SPIKE_DIR/mimeapps.list.absent" ]; then
		rm -f "$MIMEAPPS"
		log "restored: removed spike-generated $MIMEAPPS (absent before the spike)"
	elif [ -f "$SPIKE_DIR/mimeapps.list.backup" ]; then
		mkdir -p "$(dirname "$MIMEAPPS")"
		# An empty backup is a real prior state (an empty file) and is
		# restored as-is, never treated as "nothing to do".
		cp "$SPIKE_DIR/mimeapps.list.backup" "$MIMEAPPS"
		log "restored: $MIMEAPPS from backup"
	fi

	mismatch=0
	if [ -f "$SPIKE_DIR/handler-state.backup" ]; then
		while read -r scheme expected; do
			[ -n "$scheme" ] || continue
			case "$scheme" in \#*) continue ;; esac
			current=$(xdg-mime query default "$scheme" 2>/dev/null || echo none)
			if [ "$current" != "$expected" ]; then
				log "RESTORE MISMATCH: $scheme expected '$expected' found '$current'"
				mismatch=1
			fi
		done < "$SPIKE_DIR/handler-state.backup"
	fi
	if [ "$mismatch" = 0 ]; then
		log "restore verified: handler state matches the backup"
	fi
	return "$mismatch"
}

cmd_restore() {
	if [ "$SYSTEM_MODE" = 1 ]; then
		require_backup
	fi
	restore_state
	log "restore runs without a graphical session. Session-death recovery: rerun this command, or manually compare $MIMEAPPS with $SPIKE_DIR/mimeapps.list.backup (restore the copy, or delete the generated file when $SPIKE_DIR/mimeapps.list.absent exists), and remove $DESKTOP_FILE if it was spike-created."
}

# selftest runs the receiver-redaction checks on ANY host (no session, no
# Linux requirement): the exact code that ships must redact secrets and
# never log raw argv.
cmd_selftest() {
	dir=$(mktemp -d)
	write_receiver "$dir/receive.sh"

	secret="super-secret-token-123"
	url="https://spike.example/path?token=$secret"
	"$dir/receive.sh" "$url" "--some-flag"

	fail=0
	if grep -q "$secret" "$dir/received.log"; then
		echo "selftest FAIL: secret leaked into received.log"
		fail=1
	fi
	if grep -q "some-flag" "$dir/received.log"; then
		echo "selftest FAIL: raw argv leaked into received.log"
		fail=1
	fi
	if ! grep -q "query/fragment redacted" "$dir/received.log"; then
		echo "selftest FAIL: expected the redaction marker"
		fail=1
	fi
	if ! grep -q "argcount=2" "$dir/received.log"; then
		echo "selftest FAIL: expected argument count without contents"
		fail=1
	fi

	rm -rf "$dir"
	if [ "$fail" = 0 ]; then
		echo "selftest PASS: receiver redacts secrets; raw argv is never logged"
	fi
	exit "$fail"
}

first=${1:-}
if [ "$first" = "--system" ]; then
	SYSTEM_MODE=1
	shift
fi

case ${1:-} in
backup)
	require_nixos_sway
	cmd_backup
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
browser)
	require_nixos_sway
	shift
	cmd_browser "$@"
	;;
report)
	require_nixos_sway
	cmd_report
	;;
restore)
	cmd_restore
	;;
selftest)
	cmd_selftest
	;;
*)
	log "usage: $0 [--system] {backup|install|probe URL|browser DESKTOP-ID URL|report|restore|selftest}" >&2
	exit 2
	;;
esac
