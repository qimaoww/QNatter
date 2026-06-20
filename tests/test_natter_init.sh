#!/bin/sh

set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	exit 1
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

procd_log="$tmp/procd.log"
current_instance=""

procd_open_instance() {
	current_instance="$1"
	printf '%s open\n' "$current_instance" >> "$procd_log"
}

procd_set_param() {
	printf '%s set' "$current_instance" >> "$procd_log"
	printf ' %s' "$@" >> "$procd_log"
	printf '\n' >> "$procd_log"
}

procd_append_param() {
	printf '%s append' "$current_instance" >> "$procd_log"
	printf ' %s' "$@" >> "$procd_log"
	printf '\n' >> "$procd_log"
}

procd_close_instance() {
	printf '%s close\n' "$current_instance" >> "$procd_log"
	current_instance=""
}

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
	"$callback" wan_qb
}

config_get_bool() {
	config_get "$@"
}

config_get() {
	local __var="$1"
	local section="$2"
	local option="$3"
	local default="${4:-}"
	local value="$default"

	case "$section:$option" in
		global:enabled) value="1" ;;
		wan_ct:enabled) value="1" ;;
		wan_ct:label) value="Telecom" ;;
		wan_ct:protocol) value="tcp" ;;
		wan_ct:network) value="wan" ;;
		wan_ct:bind_value) value="pppoe-wan" ;;
		wan_ct:forward_method) value="none" ;;
		wan_ct:target_ip) value="10.10.10.10" ;;
		wan_ct:target_port) value="51413" ;;
		wan_cm:enabled) value="1" ;;
		wan_cm:label) value="Mobile" ;;
		wan_cm:protocol) value="udp" ;;
		wan_cm:network) value="wan2" ;;
		wan_cm:bind_value) value="eth1.2" ;;
		wan_cm:forward_method) value="none" ;;
		wan_cm:target_ip) value="10.10.10.20" ;;
		wan_cm:target_port) value="51414" ;;
		wan_qb:enabled) value="1" ;;
		wan_qb:label) value="qBittorrent" ;;
		wan_qb:protocol) value="tcp" ;;
		wan_qb:network) value="wan" ;;
		wan_qb:bind_value) value="pppoe-qb" ;;
		wan_qb:forward_method) value="none" ;;
		wan_qb:target_ip) value="10.10.10.99" ;;
		wan_qb:target_port) value="40000" ;;
		wan_qb:qbittorrent_forward) value="1" ;;
		wan_qb:qbittorrent_target_ip) value="10.10.10.30" ;;
		wan_qb:qbittorrent_target_port) value="51415" ;;
		wan_qb:qbittorrent_forward_method) value="nftables" ;;
	esac

	eval "$__var=\$value"
}

config_list_foreach() {
	return 0
}
EOF

cat > "$tmp/network.sh" <<'EOF'
# Intentionally empty. natter.init must not need network_get_device to bind.
EOF

NATTER_FUNCTIONS_SH="$tmp/functions.sh"
NATTER_NETWORK_SH="$tmp/network.sh"
NATTER_COMMON_SH="$ROOT/natter/files/natter-common.sh"
NATTER_QBITTORRENT_SH="$ROOT/natter/files/natter-qbittorrent.sh"
NATTER_RUN_DIR="$tmp/run"
NATTER_LOG_DIR="$tmp/log"
export NATTER_FUNCTIONS_SH NATTER_NETWORK_SH NATTER_COMMON_SH NATTER_QBITTORRENT_SH NATTER_RUN_DIR NATTER_LOG_DIR

. "$ROOT/natter/files/natter.init"
start_service

grep -Fqx 'wan_ct append command -i' "$procd_log" || fail "Telecom instance did not pass -i"
grep -Fqx 'wan_ct append command pppoe-wan' "$procd_log" || fail "Telecom instance did not bind pppoe-wan"
grep -Fqx 'wan_cm append command -i' "$procd_log" || fail "Mobile instance did not pass -i"
grep -Fqx 'wan_cm append command eth1.2' "$procd_log" || fail "Mobile instance did not bind eth1.2"
grep -Fqx 'wan_qb append command -i' "$procd_log" || fail "qBittorrent instance did not pass -i"
grep -Fqx 'wan_qb append command pppoe-qb' "$procd_log" || fail "qBittorrent instance did not bind pppoe-qb"
grep -Fqx 'wan_qb append command -m' "$procd_log" || fail "qBittorrent instance did not pass forwarding method flag"
grep -Fqx 'wan_qb append command nftables' "$procd_log" || fail "qBittorrent instance did not pass nftables forwarding"
grep -Fqx 'wan_qb append command -t' "$procd_log" || fail "qBittorrent instance did not pass target IP flag"
grep -Fqx 'wan_qb append command 10.10.10.30' "$procd_log" || fail "qBittorrent instance did not pass target IP"
grep -Fqx 'wan_qb append command -p' "$procd_log" || fail "qBittorrent instance did not pass target port flag"
grep -Fqx 'wan_qb append command 51415' "$procd_log" || fail "qBittorrent instance did not pass target port"

if grep -Fq 'wan_ct append command eth1.2' "$procd_log"; then
	fail "Telecom instance received Mobile bind value"
fi

if grep -Fq 'wan_cm append command pppoe-wan' "$procd_log"; then
	fail "Mobile instance received Telecom bind value"
fi

grep -Fqx 'wan_ct set env NATTER_INSTANCE=wan_ct' "$procd_log" || fail "Telecom instance env missing"
grep -Fqx 'wan_cm set env NATTER_INSTANCE=wan_cm' "$procd_log" || fail "Mobile instance env missing"
grep -Fqx "wan_ct set env NATTER_STATUS_FILE=$tmp/run/wan_ct.json" "$procd_log" || fail "Telecom status file env missing"
grep -Fqx "wan_cm set env NATTER_STATUS_FILE=$tmp/run/wan_cm.json" "$procd_log" || fail "Mobile status file env missing"

printf 'natter init checks passed\n'
