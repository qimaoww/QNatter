#!/bin/sh

set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	exit 1
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

log_dir="$tmp/log"
mkdir -p "$log_dir"
log_file="$log_dir/wan_ct.log"
escaped_log_file="$log_dir/wan_x2dct.log"

i=1
while [ "$i" -le 25 ]; do
	printf 'line-%02d\n' "$i"
	i=$((i + 1))
done > "$log_file"
printf 'dash instance log\n' > "$escaped_log_file"

output="$(
	QNATTER_COMMON_SH="$ROOT/qnatter/files/qnatter-common.sh" \
	QNATTER_LOG_DIR="$log_dir" \
	"$ROOT/luci-app-qnatter/root/usr/libexec/qnatter-log" read wan_ct 2
)"

line_count="$(printf '%s\n' "$output" | sed '/^$/d' | wc -l | tr -d ' ')"
[ "$line_count" = "20" ] || fail "read should clamp small line counts to 20, got $line_count"
printf '%s\n' "$output" | grep -Fqx 'line-06' || fail "read output should start at line-06"
printf '%s\n' "$output" | grep -Fqx 'line-25' || fail "read output should include final line"

dash_output="$(
	QNATTER_COMMON_SH="$ROOT/qnatter/files/qnatter-common.sh" \
	QNATTER_LOG_DIR="$log_dir" \
	"$ROOT/luci-app-qnatter/root/usr/libexec/qnatter-log" read 'wan-ct' 20
)"

printf '%s\n' "$dash_output" | grep -Fqx 'dash instance log' || \
	fail "log output must use collision-free runtime slug"

clear_output="$(
	QNATTER_COMMON_SH="$ROOT/qnatter/files/qnatter-common.sh" \
	QNATTER_LOG_DIR="$log_dir" \
	"$ROOT/luci-app-qnatter/root/usr/libexec/qnatter-log" clear wan_ct
)"

[ "$clear_output" = '{"ok":true}' ] || fail "clear output = $clear_output"
[ ! -s "$log_file" ] || fail "clear should truncate the instance log"

printf 'qnatter log checks passed\n'
