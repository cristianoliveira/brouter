#!/bin/sh
# TASK-0007 spike tooling: temporary URL-handler probe for NixOS + Sway
# (Wayland). Run INSIDE the graphical Sway session on the target NixOS
# machine. Everything is scoped to a spike workspace and fully restorable;
# nothing outside xdg-mime registrations and the workspace is touched.
#
# Usage:
#   nixos-url-spike.sh backup            record current handlers (required first)
#   nixos-url-spike.sh install           register the temporary logging handler
#   nixos-url-spike.sh probe URL         hand URL to the OS opener (xdg-open)
#   nixos-url-spike.sh browser URL DESKTOP-ID
#                                        hand URL to a specific browser desktop id
#   nixos-url-spike.sh report            write session/browser resolution facts
#   nixos-url-spike.sh restore           restore backed-up state, remove spike files
#
# Evidence lands in $BROUTER_SPIKE_DIR (default ./brouter-spike). Paste the
# received.log and report.txt into the TASK-0007 report; never assume
# process exit codes prove delivery — received.log is the receipt.
set -u

SPIKE_DIR="${BROUTER_SPIKE_DIR:-$PWD/brouter-spike}"
HANDLER_ID="brouter-spike-handler"
APPLICATIONS_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
MIMEAPPS="${XDG_CONFIG_HOME:-$HOME/.config}/mimeapps.list"
DESKTOP_FILE="$APPLICATIONS_DIR/$HANDLER_ID.desktop"

need_session() {
	if [ "$(uname -s)" != "Linux" ]; then
		echo "refusing: this is not Linux; this spike must run on the NixOS target" >&2
		exit 1
	fi
	if [ -z "${WAYLAND_DISPLAY:-}" ]; then
		echo "refusing: WAYLAND_DISPLAY is not set; a real Sway Wayland session is required" >&2
		exit 1
	fi
	if [ -z "${SWAYSOCK:-}" ]; then
		if command -v swaymsg >/dev/null 2>&1 && swaymsg -t get_version >/dev/null 2>&1; then
			echo "note: SWAYSOCK unset but swaymsg reached the compositor"
		else
			echo "refusing: no SWAYSOCK and swaymsg validation failed; this spike requires a real Sway session, not X11 or another compositor" >&2
			exit 1
		fi
	fi
	for tool in xdg-open xdg-mime; do
		command -v "$tool" >/dev/null 2>&1 || {
			echo "refusing: $tool is required and missing" >&2
			exit 1
		}
	done
}

require_backup() {
	if [ ! -f "$SPIKE_DIR/mimeapps.list.backup" ]; then
		echo "refusing: run 'backup' first; nothing may be changed without a recorded restore point" >&2
		exit 1
	fi
}

cmd_backup() {
	need_session
	mkdir -p "$SPIKE_DIR"
	rm -f "$SPIKE_DIR/mimeapps.list.absent"

	if [ -f "$MIMEAPPS" ]; then
		cp "$MIMEAPPS" "$SPIKE_DIR/mimeapps.list.backup"
	else
		# The absence itself must be tracked: xdg-mime will create the file
		# when we register defaults, and restore must remove it again.
		: > "$SPIKE_DIR/mimeapps.list.absent"
		: > "$SPIKE_DIR/mimeapps.list.backup"
		echo "note: no mimeapps.list exists yet; restore will remove the generated file"
	fi
	for scheme in x-scheme-handler/http x-scheme-handler/https; do
		echo "backup: $scheme currently handled by: $(xdg-mime query default "$scheme" 2>/dev/null || echo '(none)')"
	done
	echo "backup complete: $SPIKE_DIR"
}

cmd_install() {
	need_session
	require_backup
	mkdir -p "$SPIKE_DIR"

	if [ -e "$DESKTOP_FILE" ]; then
		cp "$DESKTOP_FILE" "$SPIKE_DIR/desktop.preexisting.backup"
		echo "refusing: $DESKTOP_FILE already exists and is not ours" >&2
		echo "a copy was saved to $SPIKE_DIR/desktop.preexisting.backup" >&2
		echo "review it, remove or rename the existing file yourself, then rerun install" >&2
		exit 1
	fi

	cat > "$SPIKE_DIR/receive.sh" <<RECEIVER
#!/bin/sh
{
	printf 'date=%s\n' "\$(date '+%Y-%m-%dT%H:%M:%S%z' 2>/dev/null || date)"
	printf 'argv:'
	for argument in "\$@"; do
		printf ' [%s]' "\$argument"
	done
	printf '\n'
	printf 'WAYLAND_DISPLAY=%s DISPLAY=%s XDG_SESSION_TYPE=%s XDG_CURRENT_DESKTOP=%s\n' \\
		"\${WAYLAND_DISPLAY:-}" "\${DISPLAY:-}" "\${XDG_SESSION_TYPE:-}" "\${XDG_CURRENT_DESKTOP:-}"
	printf 'invoked-by=%s\n' "\$(ps -o comm= -p \$PPID 2>/dev/null || echo unknown)"
} >> "$SPIKE_DIR/received.log"
RECEIVER
	chmod +x "$SPIKE_DIR/receive.sh"

	sed "s|@SPIKE_DIR@|$SPIKE_DIR|g" > "$DESKTOP_FILE" <<DESKTOP
[Desktop Entry]
Type=Application
Name=brouter spike handler
Exec=@SPIKE_DIR@/receive.sh %u
Terminal=false
NoDisplay=true
MimeType=x-scheme-handler/http;x-scheme-handler/https;
DESKTOP

	command -v desktop-file-validate >/dev/null 2>&1 && desktop-file-validate "$DESKTOP_FILE"
	update-desktop-database "$APPLICATIONS_DIR" 2>/dev/null || true

	xdg-mime default "$HANDLER_ID.desktop" x-scheme-handler/http
	xdg-mime default "$HANDLER_ID.desktop" x-scheme-handler/https

	echo "installed: $HANDLER_ID handles http/https; evidence: $SPIKE_DIR/received.log"
	echo "next: run 'probe URL' (terminal) and/or click a link in another app, then 'restore'"
}

cmd_probe() {
	need_session
	require_backup
	url=${1:-}
	case "$url" in
	https://* | http://*) ;;
	*)
		echo "usage: $0 probe https://spike.example/path" >&2
		exit 1
		;;
	esac

	xdg-open "$url"
	opener_rc=$?
	# Process exit is NOT proof of delivery (received.log is the receipt),
	# but an opener failure must stay visible in the exit status.
	echo "xdg-open exit=$opener_rc — exit is not proof of delivery; check received.log"
	exit "$opener_rc"
}

cmd_browser() {
	need_session
	require_backup
	desktop=${1:-}
	url=${2:-}
	case "$url" in
	https://* | http://*) ;;
	*)
		echo "usage: $0 browser DESKTOP-ID URL   (e.g. browser brave-browser https://spike.example/x)" >&2
		exit 1
		;;
	esac

	xdg-mime default "$desktop" x-scheme-handler/http
	xdg-mime default "$desktop" x-scheme-handler/https
	xdg-open "$url"
	opener_rc=$?
	echo "handed to $desktop (exit=$opener_rc); observe whether the browser was cold or already running"
	exit "$opener_rc"
}

cmd_report() {
	need_session
	mkdir -p "$SPIKE_DIR"
	{
		echo "== session =="
		echo "uname: $(uname -s -m)"
		echo "session: XDG_SESSION_TYPE=${XDG_SESSION_TYPE:-unset} XDG_CURRENT_DESKTOP=${XDG_CURRENT_DESKTOP:-unset} WAYLAND_DISPLAY=${WAYLAND_DISPLAY:-unset} SWAYSOCK=${SWAYSOCK:+set}"
		echo "== browser executable resolution (graphical session PATH) =="
		for browser in brave brave-browser google-chrome-stable chromium firefox; do
			printf '%s -> %s\n' "$browser" "$(command -v "$browser" || echo not-found)"
		done
		echo "== browser desktop files =="
		for entry in "$APPLICATIONS_DIR"/*.desktop; do
			[ -f "$entry" ] || continue
			grep -lE "brave|chrome|chromium|firefox" "$entry" 2>/dev/null
		done
		echo "== portals =="
		if command -v systemctl >/dev/null 2>&1; then
			systemctl --user list-units "xdg-desktop-portal*" --no-pager 2>/dev/null | head -10
		fi
	} > "$SPIKE_DIR/report.txt"
	cat "$SPIKE_DIR/report.txt"
	echo "report written: $SPIKE_DIR/report.txt"
}

cmd_restore() {
	need_session
	require_backup

	if [ -f "$SPIKE_DIR/desktop.preexisting.backup" ]; then
		cp "$SPIKE_DIR/desktop.preexisting.backup" "$DESKTOP_FILE"
		echo "restored: pre-existing $HANDLER_ID.desktop from backup"
	elif [ -f "$DESKTOP_FILE" ] && [ -f "$SPIKE_DIR/receive.sh" ]; then
		rm -f "$DESKTOP_FILE"
		echo "removed: spike-created $HANDLER_ID.desktop"
	fi
	update-desktop-database "$APPLICATIONS_DIR" 2>/dev/null || true

	if [ -f "$SPIKE_DIR/mimeapps.list.absent" ]; then
		# No mimeapps.list existed before the spike: the file was generated
		# by our registrations, so it is removed instead of restored.
		rm -f "$MIMEAPPS"
		echo "restored: removed spike-generated $MIMEAPPS (file did not exist before the spike)"
	elif [ -s "$SPIKE_DIR/mimeapps.list.backup" ]; then
		mkdir -p "$(dirname "$MIMEAPPS")"
		cp "$SPIKE_DIR/mimeapps.list.backup" "$MIMEAPPS"
		echo "restored: $MIMEAPPS from backup"
	fi

	for scheme in x-scheme-handler/http x-scheme-handler/https; do
		echo "restore check: $scheme -> $(xdg-mime query default "$scheme" 2>/dev/null || echo '(none)')"
	done
	echo "spike files kept in $SPIKE_DIR for evidence; delete the directory when the report is filed"
}

case ${1:-} in
backup) cmd_backup ;;
install) cmd_install ;;
probe) shift; cmd_probe "$@" ;;
browser) shift; cmd_browser "$@" ;;
report) cmd_report ;;
restore) cmd_restore ;;
*)
	echo "usage: $0 {backup|install|probe URL|browser DESKTOP-ID URL|report|restore}" >&2
	exit 2
	;;
esac
