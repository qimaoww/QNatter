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
nft_log="$tmp/nft.log"
ip_log="$tmp/ip.log"
uci_log="$tmp/uci.log"
current_instance=""
ROUTE_SLOT_wan_ct=2
ROUTE_SLOT_wan_cm=2
ROUTE_SLOT_wan_qb=bad
ROUTE_SLOT_wan_auto=
ROUTE_SLOT_disabled_lan=

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
	[ "$1" = "qnatter" ] || return 1
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
		wan_ct:route_slot) value="${ROUTE_SLOT_wan_ct:-}" ;;
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
		wan_cm:route_slot) value="${ROUTE_SLOT_wan_cm:-}" ;;
		wan_cm:forward_method) value="none" ;;
		wan_cm:target_ip) value="10.10.10.20" ;;
		wan_cm:target_port) value="51414" ;;
		wan_qb:enabled) value="1" ;;
		wan_qb:label) value="qBittorrent" ;;
		wan_qb:protocol) value="tcp" ;;
		wan_qb:network) value="wan" ;;
		wan_qb:bind_value) value="pppoe-qb" ;;
		wan_qb:route_slot) value="${ROUTE_SLOT_wan_qb:-}" ;;
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
		wan_auto:route_slot) value="${ROUTE_SLOT_wan_auto:-}" ;;
		wan_auto:forward_method) value="none" ;;
		wan_auto:target_ip) value="10.10.10.40" ;;
		wan_auto:target_port) value="51416" ;;
		disabled_lan:enabled) value="0" ;;
		disabled_lan:label) value="Disabled LAN" ;;
		disabled_lan:network) value="lan" ;;
		disabled_lan:bind_value) value="br-lan" ;;
		disabled_lan:route_slot) value="${ROUTE_SLOT_disabled_lan:-}" ;;
	esac

	eval "$__var=\$value"
}

config_set() {
	local section="$1"
	local option="$2"
	local value="$3"

	[ "$option" = "route_slot" ] || return 0
	eval "ROUTE_SLOT_${section}=\$value"
}

config_list_foreach() {
	local section="$1"
	local option="$2"
	local callback="$3"

	case "$section:$option" in
		stun:tcp_server)
			"$callback" tcp-global-one.example.com
			"$callback" tcp-global-one.example.com
			"$callback" tcp-global-two.example.com:80
			;;
		stun:udp_server)
			"$callback" udp-global-one.example.com
			"$callback" udp-global-one.example.com
			"$callback" udp-global-two.example.com:18000
			;;
		wan_qb:stun_server)
			"$callback" tcp-instance-qb.example.com
			"$callback" tcp-global-one.example.com
			"$callback" tcp-instance-qb.example.com
			;;
	esac
	return 0
}
EOF

cat > "$tmp/network.sh" <<'EOF'
# Intentionally empty. qnatter.init must not need network_get_device to bind.
EOF

cat > "$tmp/nft" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >> "$QNATTER_TEST_NFT_LOG"
EOF
chmod +x "$tmp/nft"

cat > "$tmp/ip" <<'EOF'
#!/bin/sh
if [ "$1 $2" = "rule show" ]; then
	cat <<'RULES'
100:	from all fwmark 0x1 lookup 100
20192:	from all fwmark 0x4e0000c0 lookup 20192
20253:	from all fwmark 0x4e0000fd lookup 20253
24000:	from all fwmark 0x4e001000 lookup 24000
RULES
	exit 0
fi
printf '%s\n' "$*" >> "$QNATTER_TEST_IP_LOG"
EOF
chmod +x "$tmp/ip"

cat > "$tmp/uci" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >> "$QNATTER_TEST_UCI_LOG"
exit 0
EOF
chmod +x "$tmp/uci"

QNATTER_FUNCTIONS_SH="$tmp/functions.sh"
QNATTER_NETWORK_SH="$tmp/network.sh"
QNATTER_COMMON_SH="$ROOT/qnatter/files/qnatter-common.sh"
QNATTER_QBITTORRENT_SH="$ROOT/qnatter/files/qnatter-qbittorrent.sh"
QNATTER_RUN_DIR="$tmp/run"
QNATTER_LOG_DIR="$tmp/log"
QNATTER_NFT_BIN="$tmp/nft"
QNATTER_IP_BIN="$tmp/ip"
QNATTER_UCI_BIN="$tmp/uci"
QNATTER_TEST_NFT_LOG="$nft_log"
QNATTER_TEST_IP_LOG="$ip_log"
QNATTER_TEST_UCI_LOG="$uci_log"
export QNATTER_FUNCTIONS_SH QNATTER_NETWORK_SH QNATTER_COMMON_SH QNATTER_QBITTORRENT_SH QNATTER_RUN_DIR QNATTER_LOG_DIR QNATTER_NFT_BIN QNATTER_IP_BIN QNATTER_UCI_BIN QNATTER_TEST_NFT_LOG QNATTER_TEST_IP_LOG QNATTER_TEST_UCI_LOG

. "$ROOT/qnatter/files/qnatter.init"
start_service
service_triggers

grep -Fqx 'flush chain ip qnatter qnatter_dnat' "$nft_log" || fail "start_service did not flush stale DNAT rules"
grep -Fqx 'flush chain ip qnatter qnatter_snat' "$nft_log" || fail "start_service did not flush stale SNAT rules"
grep -Fqx 'flush chain ip qnatter qnatter_mark' "$nft_log" || fail "start_service did not flush stale mark rules"
grep -Fqx 'rule del priority 20192' "$ip_log" || fail "start_service did not delete stale QNatter rule 20192"
grep -Fqx 'route flush table 20192' "$ip_log" || fail "start_service did not flush stale QNatter table 20192"
grep -Fqx 'rule del priority 20253' "$ip_log" || fail "start_service did not delete stale QNatter rule 20253"
grep -Fqx 'route flush table 20253' "$ip_log" || fail "start_service did not flush stale QNatter table 20253"
if grep -Fq 'priority 100' "$ip_log" || grep -Fq 'priority 24000' "$ip_log"; then
	fail "start_service deleted non-QNatter policy rules"
fi

grep -Fqx 'set qnatter.wan_cm.route_slot=0' "$uci_log" || fail "duplicate route slot was not repaired"
grep -Fqx 'set qnatter.wan_qb.route_slot=1' "$uci_log" || fail "invalid route slot was not repaired"
grep -Fqx 'set qnatter.wan_auto.route_slot=3' "$uci_log" || fail "missing route slot was not repaired"
grep -Fqx 'set qnatter.disabled_lan.route_slot=4' "$uci_log" || fail "disabled instance route slot was not prepared"
grep -Fqx 'commit qnatter' "$uci_log" || fail "route slot repair was not committed"

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
grep -Fqx 'wan_ct append command tcp-global-one.example.com' "$procd_log" || fail "TCP instance did not use global TCP STUN server"
grep -Fqx 'wan_ct append command tcp-global-two.example.com:80' "$procd_log" || fail "TCP instance did not use second global TCP STUN server"
grep -Fqx 'wan_cm append command udp-global-one.example.com' "$procd_log" || fail "UDP instance did not use global UDP STUN server"
grep -Fqx 'wan_cm append command udp-global-two.example.com:18000' "$procd_log" || fail "UDP instance did not use second global UDP STUN server"
grep -Fqx 'wan_qb append command tcp-instance-qb.example.com' "$procd_log" || fail "Instance STUN override was not used"
grep -Fqx 'wan_qb append command tcp-global-one.example.com' "$procd_log" || fail "Instance STUN fallback did not include global TCP STUN server"
grep -Fqx 'wan_qb append command tcp-global-two.example.com:80' "$procd_log" || fail "Instance STUN fallback did not include second global TCP STUN server"
[ "$(grep -Fxc 'wan_ct append command tcp-global-one.example.com' "$procd_log")" = "1" ] || fail "TCP global STUN duplicate was passed to QNatter"
[ "$(grep -Fxc 'wan_cm append command udp-global-one.example.com' "$procd_log")" = "1" ] || fail "UDP global STUN duplicate was passed to QNatter"
[ "$(grep -Fxc 'wan_qb append command tcp-instance-qb.example.com' "$procd_log")" = "1" ] || fail "Instance STUN duplicate was passed to QNatter"
[ "$(grep -Fxc 'wan_qb append command tcp-global-one.example.com' "$procd_log")" = "1" ] || fail "Instance STUN fallback duplicate was passed to QNatter"

if grep -Fq 'wan_ct append command eth1.2' "$procd_log"; then
	fail "Telecom instance received Mobile bind value"
fi
if grep -Fq 'wan_cm append command pppoe-wan' "$procd_log"; then
	fail "Mobile instance received Telecom bind value"
fi

grep -Fqx "wan_ct set env QNATTER_INSTANCE=wan_ct QNATTER_STATUS_FILE=$tmp/run/wan_ct.json QNATTER_ROUTE_MARK=0x4e000002 QNATTER_ROUTE_TABLE=20002 QNATTER_ROUTE_PRIORITY=20002" "$procd_log" || fail "Telecom instance env missing"
grep -Fqx "wan_cm set env QNATTER_INSTANCE=wan_cm QNATTER_STATUS_FILE=$tmp/run/wan_cm.json QNATTER_ROUTE_MARK=0x4e000000 QNATTER_ROUTE_TABLE=20000 QNATTER_ROUTE_PRIORITY=20000" "$procd_log" || fail "Mobile instance env missing"
grep -Fqx "wan_qb set env QNATTER_INSTANCE=wan_qb QNATTER_STATUS_FILE=$tmp/run/wan_qb.json QNATTER_ROUTE_MARK=0x4e000001 QNATTER_ROUTE_TABLE=20001 QNATTER_ROUTE_PRIORITY=20001" "$procd_log" || fail "qBittorrent instance env missing"
grep -Fqx "wan_auto set env QNATTER_INSTANCE=wan_auto QNATTER_STATUS_FILE=$tmp/run/wan_auto.json QNATTER_ROUTE_MARK=0x4e000003 QNATTER_ROUTE_TABLE=20003 QNATTER_ROUTE_PRIORITY=20003" "$procd_log" || fail "Auto WAN instance env missing"

grep -Fq "QNATTER_LOG_FILE='$tmp/log/wan_ct.log'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing instance log file"
grep -Fq "QNATTER_AUTO_FIREWALL='1'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing auto firewall flag"
grep -Fq "QNATTER_FIREWALL_SECTION='qnatter_wan_ct'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing firewall section"
grep -Fq "QNATTER_FIREWALL_NAME='QNatter wan_ct'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing firewall name"
grep -Fq "QNATTER_FIREWALL_SRC='wan'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing firewall source zone"
grep -Fq "CLOUDFLARE_SRV_ENABLED='1'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing Cloudflare SRV flag"
grep -Fq "CLOUDFLARE_API_URL='https://api.cloudflare.com/client/v4/zones/zone/dns_records/record'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing Cloudflare API URL"
grep -Fq "CLOUDFLARE_API_TOKEN='cf-secret'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing Cloudflare API token"
grep -Fq "CLOUDFLARE_ZONE_ID='zone-selected'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing Cloudflare zone ID"
grep -Fq "CLOUDFLARE_RECORD_ID='record-selected'" "$tmp/run/wan_ct.env" || fail "Telecom notify env missing Cloudflare record ID"
grep -Fq "QNATTER_AUTO_FIREWALL='0'" "$tmp/run/wan_cm.env" || fail "Mobile notify env should disable auto firewall by default"
grep -Fq "CLOUDFLARE_SRV_ENABLED='0'" "$tmp/run/wan_cm.env" || fail "Mobile notify env should disable Cloudflare SRV by default"
grep -Fq "QNATTER_FIREWALL_DEST='lan'" "$tmp/run/wan_qb.env" || fail "qBittorrent notify env missing firewall destination zone"

grep -Fqx 'reload qnatter' "$trigger_log" || fail "config reload trigger missing"
grep -Fqx 'interface interface.* wan3 /etc/init.d/qnatter reload' "$trigger_log" || fail "unbound instance network trigger missing"
if grep -Fqx 'interface interface.* wan /etc/init.d/qnatter reload' "$trigger_log"; then
	fail "bound wan instance must not register network trigger"
fi
if grep -Fqx 'interface interface.* wan2 /etc/init.d/qnatter reload' "$trigger_log"; then
	fail "bound wan2 instance must not register network trigger"
fi
if grep -Fq ' lan ' "$trigger_log"; then
	fail "disabled instance must not register interface trigger"
fi

printf 'qnatter init checks passed\n'
