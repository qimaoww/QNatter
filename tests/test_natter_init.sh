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
trigger_log="$tmp/triggers.log"
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

procd_add_reload_trigger() {
	printf 'reload' >> "$trigger_log"
	printf ' %s' "$@" >> "$trigger_log"
	printf '\n' >> "$trigger_log"
}

procd_add_interface_trigger() {
	printf 'interface' >> "$trigger_log"
	printf ' %s' "$@" >> "$trigger_log"
	printf '\n' >> "$trigger_log"
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
	"$callback" wan_auto
	"$callback" disabled_lan
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
		wan_ct:auto_firewall) value="1" ;;
		wan_ct:target_ip) value="10.10.10.10" ;;
		wan_ct:target_port) value="51413" ;;
		wan_ct:cloudflare_enabled) value="1" ;;
		wan_ct:cloudflare_api_url) value="https://api.cloudflare.com/client/v4/zones/zone/dns_records/record" ;;
		wan_ct:cloudflare_api_token) value="cf-secret" ;;
		wan_ct:cloudflare_zone_id) value="zone-selected" ;;
		wan_ct:cloudflare_record_id) value="record-selected" ;;
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
		wan_auto:enabled) value="1" ;;
		wan_auto:label) value="Auto WAN" ;;
		wan_auto:protocol) value="tcp" ;;
		wan_auto:network) value="wan3" ;;
		wan_auto:bind_value) value="" ;;
		wan_auto:forward_method) value="none" ;;
		wan_auto:target_ip) value="10.10.10.40" ;;
		wan_auto:target_port) value="51416" ;;
		disabled_lan:enabled) value="0" ;;
		disabled_lan:label) value="Disabled LAN" ;;
		disabled_lan:network) value="lan" ;;
		disabled_lan:bind_value) value="br-lan" ;;
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
service_triggers

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

grep -Fq "NATTER_LOG_FILE='$tmp/log/wan_ct.log'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing instance log file"
grep -Fq "NATTER_AUTO_FIREWALL='1'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing auto firewall flag"
grep -Fq "NATTER_FIREWALL_SECTION='natter_wan_ct'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing firewall section"
grep -Fq "NATTER_FIREWALL_NAME='Natter wan_ct'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing firewall name"
grep -Fq "NATTER_FIREWALL_SRC='wan'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing firewall source zone"
grep -Fq "CLOUDFLARE_SRV_ENABLED='1'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing Cloudflare SRV flag"
grep -Fq "CLOUDFLARE_API_URL='https://api.cloudflare.com/client/v4/zones/zone/dns_records/record'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing Cloudflare API URL"
grep -Fq "CLOUDFLARE_API_TOKEN='cf-secret'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing Cloudflare API token"
grep -Fq "CLOUDFLARE_ZONE_ID='zone-selected'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing Cloudflare zone ID"
grep -Fq "CLOUDFLARE_RECORD_ID='record-selected'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing Cloudflare record ID"
grep -Fq "NATTER_AUTO_FIREWALL='0'" "$tmp/run/wan_cm.env" || fail "Mobile notify env should disable auto firewall by default"
grep -Fq "CLOUDFLARE_SRV_ENABLED='0'" "$tmp/run/wan_cm.env" || fail "Mobile notify env should disable Cloudflare SRV by default"
grep -Fq "NATTER_FIREWALL_DEST='lan'" "$tmp/run/wan_qb.env" || fail "qBittorrent notify env missing firewall destination zone"

grep -Fqx 'reload natter' "$trigger_log" || fail "config reload trigger missing"
grep -Fqx 'interface interface.* wan3 /etc/init.d/natter reload' "$trigger_log" || fail "unbound instance network trigger missing"
if grep -Fqx 'interface interface.* wan /etc/init.d/natter reload' "$trigger_log"; then
	fail "bound wan instance must not register network trigger"
fi
if grep -Fqx 'interface interface.* wan2 /etc/init.d/natter reload' "$trigger_log"; then
	fail "bound wan2 instance must not register network trigger"
fi
if grep -Fq ' lan ' "$trigger_log"; then
	fail "disabled instance must not register interface trigger"
fi

printf 'natter init checks passed\n'
