#!/bin/sh
set -eu

REPO="rschoch/lazynote"
BINARY="lazynote"

version="${VERSION:-latest}"

if [ -n "${INSTALL_DIR:-}" ]; then
	install_dir="$INSTALL_DIR"
elif [ -n "${HOME:-}" ]; then
	install_dir="$HOME/.local/bin"
else
	install_dir="/usr/local/bin"
fi

usage() {
	cat <<EOF
Install lazynote from the latest GitHub release.

Usage:
  sh install.sh [--version v0.1.0] [--dir /path/to/bin]

Options:
  --version VERSION  Install a specific release tag. Defaults to latest.
  --dir DIR          Install directory. Defaults to ~/.local/bin.
  -h, --help         Show this help.

Environment:
  VERSION            Same as --version.
  INSTALL_DIR        Same as --dir.
EOF
}

log() {
	printf '%s\n' "$*"
}

fail() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

while [ "$#" -gt 0 ]; do
	case "$1" in
		--version)
			[ "$#" -ge 2 ] || fail "--version requires a value"
			version="$2"
			shift 2
			;;
		--dir)
			[ "$#" -ge 2 ] || fail "--dir requires a value"
			install_dir="$2"
			shift 2
			;;
		-h | --help)
			usage
			exit 0
			;;
		*)
			fail "unknown argument: $1"
			;;
	esac
done

download_stdout() {
	url="$1"
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO- "$url"
	else
		fail "curl or wget is required"
	fi
}

download_file() {
	url="$1"
	dest="$2"
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL -o "$dest" "$url"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$dest" "$url"
	else
		fail "curl or wget is required"
	fi
}

resolve_version() {
	if [ "$version" = "latest" ]; then
		download_stdout "https://api.github.com/repos/${REPO}/releases/latest" |
			sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
			head -n 1
	else
		case "$version" in
			v*) printf '%s\n' "$version" ;;
			*) printf 'v%s\n' "$version" ;;
		esac
	fi
}

detect_os() {
	os="$(uname -s)"
	case "$os" in
		Linux) printf 'linux\n' ;;
		Darwin) printf 'darwin\n' ;;
		*) fail "unsupported OS: $os" ;;
	esac
}

detect_arch() {
	arch="$(uname -m)"
	case "$arch" in
		x86_64 | amd64) printf 'amd64\n' ;;
		arm64 | aarch64) printf 'arm64\n' ;;
		*) fail "unsupported architecture: $arch" ;;
	esac
}

verify_checksum() {
	checksums="$1"
	artifact="$2"
	checksum_line="$3"

	if ! awk -v file="$artifact" '$2 == file { print; found = 1 } END { exit found ? 0 : 1 }' "$checksums" >"$checksum_line"; then
		fail "checksum entry not found for $artifact"
	fi

	if command -v sha256sum >/dev/null 2>&1; then
		if ! (cd "$tmp_dir" && sha256sum -c "$(basename "$checksum_line")" >/dev/null); then
			fail "checksum verification failed for $artifact"
		fi
	elif command -v shasum >/dev/null 2>&1; then
		if ! (cd "$tmp_dir" && shasum -a 256 -c "$(basename "$checksum_line")" >/dev/null); then
			fail "checksum verification failed for $artifact"
		fi
	else
		log "No SHA-256 tool found; skipping checksum verification."
	fi
}

tag="$(resolve_version)"
[ -n "$tag" ] || fail "could not resolve latest release"

release_version="${tag#v}"
os="$(detect_os)"
arch="$(detect_arch)"
archive="${BINARY}_${release_version}_${os}_${arch}.tar.gz"
base_url="https://github.com/${REPO}/releases/download/${tag}"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

log "Installing ${BINARY} ${tag} for ${os}/${arch}"
download_file "${base_url}/${archive}" "$tmp_dir/$archive"
download_file "${base_url}/checksums.txt" "$tmp_dir/checksums.txt"
verify_checksum "$tmp_dir/checksums.txt" "$archive" "$tmp_dir/checksum-line"

if ! tar -xzf "$tmp_dir/$archive" -C "$tmp_dir"; then
	fail "could not extract $archive"
fi

[ -f "$tmp_dir/$BINARY" ] || fail "archive did not contain $BINARY"

if ! mkdir -p "$install_dir"; then
	fail "could not create install directory: $install_dir"
fi

if ! touch "$install_dir/.lazynote-install-test" 2>/dev/null; then
	fail "install directory is not writable: $install_dir"
fi
rm -f "$install_dir/.lazynote-install-test"

if command -v install >/dev/null 2>&1; then
	install -m 0755 "$tmp_dir/$BINARY" "$install_dir/$BINARY"
else
	cp "$tmp_dir/$BINARY" "$install_dir/$BINARY"
	chmod 0755 "$install_dir/$BINARY"
fi

log "Installed ${BINARY} to ${install_dir}/${BINARY}"

case ":$PATH:" in
	*":$install_dir:"*) ;;
	*) log "Add ${install_dir} to PATH to run ${BINARY} from any directory." ;;
esac

"$install_dir/$BINARY" --version
