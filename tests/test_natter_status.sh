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
proc_dir="$tmp/proc"
mkdir -p "$run_dir"
mkdir -p "$proc_dir/100" "$proc_dir/101"

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
	"$callback" dot.name
	"$callback" dotXname
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
		wan_cm:enabled) value="true" ;;
		wan_cm:network) value="wan2" ;;
		wan_cm:bind_value) value="eth1.2" ;;
		wan_cm:protocol) value="udp" ;;
		wan_cm:qbittorrent_enabled) value="yes" ;;
		wan_cm:qbittorrent_forward) value="on" ;;
		dot.name:label) value="Dotted WAN" ;;
		dot.name:enabled) value="1" ;;
		dot.name:network) value="wan3" ;;
		dot.name:bind_value) value="pppoe-wan.3" ;;
		dot.name:protocol) value="tcp" ;;
		dotXname:label) value="Regex Trap WAN" ;;
		dotXname:enabled) value="1" ;;
		dotXname:network) value="wan4" ;;
		dotXname:bind_value) value="pppoe-wan4" ;;
		dotXname:protocol) value="tcp" ;;
	esac

	eval "$__var=\$value"
}
EOF

printf 'NATTER_INSTANCE=dotXname\0' > "$proc_dir/100/environ"
printf 'NATTER_STATUS_FILE=%s/wan_ct.json\0' "$run_dir" > "$proc_dir/101/environ"

cat > "$run_dir/wan_ct.json" <<'EOF'
{"instance":"wan_ct","protocol":"tcp","inner_ip":"10.10.10.10","inner_port":51413,"outer_ip":"203.0.113.10","outer_port":62000,"updated_at":"2026-06-20 04:00:00","message":"mapped \"quote\" \\ path\nnext\tend \u003cwan\u003e\u0026ok"}
EOF

stderr="$tmp/status.err"
if ! output="$(
	NATTER_FUNCTIONS_SH="$tmp/functions.sh" \
	NATTER_COMMON_SH="$ROOT/natter/files/natter-common.sh" \
	NATTER_RUN_DIR="$run_dir" \
	NATTER_PROC_DIR="$proc_dir" \
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
	*'"name":"wan_ct"'*'"bind_value":"pppoe-wan"'*) : ;;
	*) fail "status output is missing Telecom instance: $output" ;;
esac

case "$output" in
	*'"name":"wan_cm"'*'"bind_value":"eth1.2"'*) : ;;
	*) fail "status output is missing Mobile instance: $output" ;;
esac
if printf '%s\n' "$output" | grep -Eq '"label":|"group":'; then
	fail "status output should not expose label/group fields: $output"
fi

printf '%s\n' "$output" | grep -Fq '"name":"wan_cm"' || \
	fail "status output is missing Mobile instance name: $output"
printf '%s\n' "$output" | grep -Fq '"enabled":true' || \
	fail "status output must parse UCI-style enabled boolean: $output"
printf '%s\n' "$output" | grep -Fq '"qbittorrent_enabled":true' || \
	fail "status output must parse UCI-style qBittorrent boolean: $output"
printf '%s\n' "$output" | grep -Fq '"qbittorrent_forward":true' || \
	fail "status output must parse UCI-style qBittorrent forwarding boolean: $output"

printf '%s\n' "$output" | grep -Fq '"inner_ip":"10.10.10.10"' || \
	fail "status output is missing Telecom inner IP: $output"
printf '%s\n' "$output" | grep -Fq '"inner_port":51413' || \
	fail "status output is missing Telecom inner port: $output"
printf '%s\n' "$output" | grep -Fq '"outer_ip":"203.0.113.10"' || \
	fail "status output is missing Telecom outer IP: $output"
printf '%s\n' "$output" | grep -Fq '"outer_port":62000' || \
	fail "status output is missing Telecom outer port: $output"
printf '%s\n' "$output" | grep -Fq '"message":"mapped \"quote\" \\ path\nnext\tend <wan>&ok"' || \
	fail "status output must preserve escaped runtime strings: $output"

case "$output" in
	*'"name":"wan_ct"'*'"running":true'*'"network":"wan"'*) : ;;
	*) fail "status output must detect runtime status file proc instance: $output" ;;
esac

case "$output" in
	*'"name":"wan_cm"'*'"inner_port":0'*'"outer_port":0'*) : ;;
	*) fail "status output must use zero ports when runtime status is absent: $output" ;;
esac

case "$output" in
	*'"name":"dot.name"'*'"running":false'*) : ;;
	*) fail "status output must not treat regex-like instance names as running: $output" ;;
esac

case "$output" in
	*'"name":"dotXname"'*'"running":true'*) : ;;
	*) fail "status output must detect exact fake proc instance: $output" ;;
esac

printf 'natter status checks passed\n'
