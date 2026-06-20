#!/bin/sh

set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	exit 1
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

config_file="$tmp/natter"
default_file="$tmp/natter.default"

printf 'default-config\n' > "$default_file"

NATTER_UCI_CONFIG="$config_file" \
NATTER_UCI_DEFAULT="$default_file" \
	sh "$ROOT/natter/files/natter.uci-default"

[ "$(cat "$config_file")" = "default-config" ] || fail "uci-defaults did not initialize missing config"

printf 'user-config\n' > "$config_file"

NATTER_UCI_CONFIG="$config_file" \
NATTER_UCI_DEFAULT="$default_file" \
	sh "$ROOT/natter/files/natter.uci-default"

[ "$(cat "$config_file")" = "user-config" ] || fail "uci-defaults overwrote existing user config"

rm -f "$config_file" "$default_file"

NATTER_UCI_CONFIG="$config_file" \
NATTER_UCI_DEFAULT="$default_file" \
	sh "$ROOT/natter/files/natter.uci-default"

[ ! -e "$config_file" ] || fail "uci-defaults created config when default was missing"

printf 'natter uci-default checks passed\n'
