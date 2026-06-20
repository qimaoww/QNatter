#!/bin/sh

set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	exit 1
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
run_dir="$tmp/run"
mkdir -p "$run_dir"

cat > "$tmp/functions.sh" <<'EOF'
config_load() {
	[ "$1" = "natter" ] || return 1
}

config_foreach() {
	local callback="$1"
	local type="$2"
	[ "$type" = "instance" ] || return 0
	"$callback" wan_ct
	"$callback" wan_cm
}

config_get() {
	local __var="$1"
	local section="$2"
	local option="$3"
	local default="${4:-}"
	local value="$default"

	case "$section:$option" in
		wan_ct:label) value="Telecom WAN" ;;
		wan_ct:enabled) value="1" ;;
		wan_ct:network) value="wan" ;;
		wan_ct:bind_value) value="pppoe-wan" ;;
		wan_ct:protocol) value="tcp" ;;
		wan_cm:label) value="Mobile WAN" ;;
		wan_cm:enabled) value="1" ;;
		wan_cm:network) value="wan2" ;;
		wan_cm:bind_value) value="eth1.2" ;;
		wan_cm:protocol) value="udp" ;;
	esac

	eval "$__var=\$value"
}
EOF

cat > "$run_dir/wan_ct.json" <<'EOF'
{"instance":"wan_ct","protocol":"tcp","inner_ip":"10.10.10.10","inner_port":51413,"outer_ip":"203.0.113.10","outer_port":62000,"updated_at":"2026-06-20 04:00:00","message":"mapped"}
EOF

stderr="$tmp/status.err"
if ! output="$(
	NATTER_FUNCTIONS_SH="$tmp/functions.sh" \
	NATTER_COMMON_SH="$ROOT/natter/files/natter-common.sh" \
	NATTER_RUN_DIR="$run_dir" \
	"$ROOT/luci-app-natter/root/usr/libexec/natter-status"
)" 2>"$stderr"; then
	cat "$stderr" >&2
	fail "natter-status returned an error"
fi

if [ -s "$stderr" ]; then
	cat "$stderr" >&2
	fail "natter-status must not write proc scan races to stderr"
fi

case "$output" in
	*'"name":"wan_ct"'*'"label":"Telecom WAN"'*'"bind_value":"pppoe-wan"'*) : ;;
	*) fail "status output is missing Telecom instance: $output" ;;
esac

case "$output" in
	*'"name":"wan_cm"'*'"label":"Mobile WAN"'*'"bind_value":"eth1.2"'*) : ;;
	*) fail "status output is missing Mobile instance: $output" ;;
esac

case "$output" in
	*'"inner_ip":"10.10.10.10"'*'"inner_port":51413'*'"outer_ip":"203.0.113.10"'*'"outer_port":62000'*'"message":"mapped"'*) : ;;
	*) fail "status output is missing Telecom runtime mapping: $output" ;;
esac

case "$output" in
	*'"name":"wan_cm"'*'"inner_port":0'*'"outer_port":0'*) : ;;
	*) fail "status output must use zero ports when runtime status is absent: $output" ;;
esac

printf 'natter status checks passed\n'
