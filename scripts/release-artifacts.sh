#!/usr/bin/env bash
# Build deterministic, supported release archives from a validated tag.
#
# This helper creates artifacts only. The workflow is responsible for the
# draft GitHub release; no command here creates tags, releases, or installs.
set -euo pipefail

repo_root=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo_root"

usage() {
	cat >&2 <<'EOF'
usage:
  release-artifacts.sh validate-tag TAG
  release-artifacts.sh build TAG TARGET OUTPUT_DIR
  release-artifacts.sh manifest TAG ARTIFACT_DIR

TARGET is one of: linux-x86_64, linux-aarch64, darwin-arm64, darwin-handler-arm64.
EOF
}

die() {
	echo "release-artifacts: $*" >&2
	exit 1
}

validate_tag() {
	local tag=${1-}
	[[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] ||
		die "tag must match vX.Y.Z (got: $tag)"
	printf '%s\n' "${tag#v}"
}

archive_tree() {
	local source=$1 archive=$2
	python3 - "$source" "$archive" <<'PY'
import gzip
import os
import sys
import tarfile

source = os.path.abspath(sys.argv[1])
archive = os.path.abspath(sys.argv[2])
parent = os.path.dirname(source)

with open(archive, "wb") as raw:
    with gzip.GzipFile(fileobj=raw, mode="wb", filename="", mtime=0) as compressed:
        with tarfile.open(fileobj=compressed, mode="w", format=tarfile.PAX_FORMAT) as tar:
            paths = [source]
            for root, dirs, files in os.walk(source):
                dirs.sort()
                files.sort()
                paths.extend(os.path.join(root, name) for name in dirs + files)
            for path in paths:
                relative = os.path.relpath(path, parent)
                info = tar.gettarinfo(path, arcname=relative)
                info.uid = 0
                info.gid = 0
                info.uname = ""
                info.gname = ""
                info.mtime = 0
                info.pax_headers = {}
                if info.isreg():
                    with open(path, "rb") as content:
                        tar.addfile(info, content)
                else:
                    tar.addfile(info)
PY
}

verify_archive() {
	local archive=$1 root=$2 version=$3
	python3 - "$archive" "$root" "$version" <<'PY'
import sys
import tarfile

archive, expected_root, version = sys.argv[1:]
with tarfile.open(archive, "r:gz") as tar:
    names = tar.getnames()
    required = {
        expected_root,
        expected_root + "/VERSION",
    }
    if not required.issubset(names):
        raise SystemExit(f"archive missing required entries: {archive}")
    if any(name.startswith("/") or ".." in name.split("/") for name in names):
        raise SystemExit(f"archive contains unsafe path: {archive}")
    stamped = tar.extractfile(expected_root + "/VERSION")
    if stamped is None or stamped.read().decode().strip() != version:
        raise SystemExit(f"archive has wrong VERSION: {archive}")
PY
}

build_cli() (
	local version=$1 target=$2 output=$3 goos=$4 goarch=$5 artifact=$6
	local stage root archive
	stage=$(mktemp -d)
	trap 'rm -rf "$stage"' EXIT
	root="brouter-${version}-${target}"
	archive="$output/${artifact}.tar.gz"
	mkdir -p "$output" "$stage/$root"
	GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
		go build -trimpath -ldflags "-X main.version=$version" \
		-o "$stage/$root/brouter" ./cmd/brouter
	printf '%s\n' "$version" > "$stage/$root/VERSION"
	archive_tree "$stage/$root" "$archive"
	verify_archive "$archive" "$root" "$version"
	echo "$archive"
)

build_handler() (
	local version=$1 output=$2
	local stage root archive app
	stage=$(mktemp -d)
	trap 'rm -rf "$stage"' EXIT
	root="brouter-handler-${version}-darwin-arm64"
	archive="$output/brouter-handler-v${version}-darwin-arm64.tar.gz"
	mkdir -p "$output"
	RELEASE_VERSION="$version" bash scripts/build-macos-handler.sh
	app="dist/BrouterHandler.app"
	[[ -d "$app" ]] || die "macOS helper did not produce $app"
	mkdir -p "$stage/$root"
	cp -R "$app" "$stage/$root/BrouterHandler.app"
	printf '%s\n' "$version" > "$stage/$root/VERSION"
	archive_tree "$stage/$root" "$archive"
	verify_archive "$archive" "$root" "$version"
	echo "$archive"
)

build() {
	local tag=$1 target=$2 output=$3
	local version
	version=$(validate_tag "$tag")
	[[ -n "$target" && -n "$output" ]] || die "build requires TAG TARGET OUTPUT_DIR"
	case "$target" in
	linux-x86_64)
		build_cli "$version" "$target" "$output" linux amd64 "brouter-${tag}-linux-x86_64" ;;
	linux-aarch64)
		build_cli "$version" "$target" "$output" linux arm64 "brouter-${tag}-linux-aarch64" ;;
	darwin-arm64)
		build_cli "$version" "$target" "$output" darwin arm64 "brouter-${tag}-darwin-arm64" ;;
	darwin-handler-arm64)
		[[ "$(uname -s)" == Darwin ]] || die "macOS handler must build on Darwin"
		build_handler "$version" "$output" ;;
	*)
		die "unsupported target: $target" ;;
	esac
}

manifest() {
	local tag=$1 dir=$2 version
	version=$(validate_tag "$tag")
	[[ -d "$dir" ]] || die "artifact directory does not exist: $dir"
	local expected=(
		"brouter-${tag}-linux-x86_64.tar.gz"
		"brouter-${tag}-linux-aarch64.tar.gz"
		"brouter-${tag}-darwin-arm64.tar.gz"
		"brouter-handler-${tag}-darwin-arm64.tar.gz"
	)
	local name
	for name in "${expected[@]}"; do
		[[ -f "$dir/$name" ]] || die "missing expected artifact: $name"
	done
	shopt -s nullglob
	local files=("$dir"/*.tar.gz)
	shopt -u nullglob
	[[ "${#files[@]}" -eq "${#expected[@]}" ]] || die "unexpected archive in $dir"
	( cd "$dir" && sha256sum "${expected[@]}" > SHA256SUMS )
	printf 'Brouter %s release artifacts\n' "$tag"
	cat "$dir/SHA256SUMS"
}

[[ $# -ge 2 ]] || { usage; exit 2; }
command=$1
case "$command" in
validate-tag)
	[[ $# -eq 2 ]] || { usage; exit 2; }
	validate_tag "$2" ;;
build)
	[[ $# -eq 4 ]] || { usage; exit 2; }
build "$2" "$3" "$4" ;;
manifest)
	[[ $# -eq 3 ]] || { usage; exit 2; }
manifest "$2" "$3" ;;
*)
	usage
	exit 2
	;;
esac
