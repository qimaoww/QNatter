#!/bin/sh
# Verify that a freshly built binary really targets the architecture the
# package build asked for.
#
# This runs as a separate script on purpose: inside `define Build/Compile` a
# doubled-dollar reference reaches the shell only after several rounds of make
# expansion (OpenWrt wraps package recipes in layers of define), so inline
# shell variables silently collapse to their last letters and inline command
# substitution becomes the empty string. Keeping the quoting here means shell
# variables are simply shell variables.
#
# An empty GOARCH used to make `go build` fall back to the host architecture
# and still exit 0, which shipped x86_64 binaries for every target.
#
# usage: check-goarch.sh <binary> <expected-goarch> <openwrt-arch>
set -eu

binary="$1"
expected="$2"
arch="$3"

[ -n "$expected" ] || {
	echo "qnatter: cannot map target architecture '$arch' to GOARCH" >&2
	exit 1
}

out="$(go version -m "$binary" 2>&1)" || {
	echo "qnatter: 'go version -m $binary' failed; it said:" >&2
	printf '%s\n' "$out" >&2
	exit 1
}

# `go version -m` prints tab-separated "build" records such as
#	build	GOARCH=amd64
# so the value is a field of its own. Scan fields instead of parsing by
# position: the leading tab makes $1 empty and the record set varies by Go
# version. Never go back to extracting the value with a positional
# grep, because an unmatched pattern used to yield the empty string without
# failing the build.
built="$(
	printf '%s\n' "$out" | awk -F'\t' '
		{
			for (i = 1; i <= NF; i++)
				if ($i ~ /^GOARCH=/) {
					sub(/^GOARCH=/, "", $i)
					print $i
					exit
				}
		}
	'
)"

[ -n "$built" ] || {
	echo "qnatter: cannot determine the GOARCH of $binary; 'go version -m' said:" >&2
	printf '%s\n' "$out" >&2
	exit 1
}

[ "$built" = "$expected" ] || {
	echo "qnatter: built binary GOARCH=$built, expected $expected (target ARCH=$arch)" >&2
	exit 1
}

echo "qnatter: built binary GOARCH=$built matches target ARCH=$arch"
