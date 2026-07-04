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
service_log="$tmp/service.log"
trigger_log="$tmp/triggers.log"
nft_log="$tmp/nft.log"
ip_log="$tmp/ip.log"
current_instance=""
GLOBAL_ENABLED=1
HOT_RELOAD=1
BIND_VALUE="pppoe-wan"
CF_TOKEN="token-one"
WAN_ADDR="100.64.1.1"

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
	printf 'reload %s\n' "$*" >> "$trigger_log"
}

procd_add_interface_trigger() {
	printf 'interface %s\n' "$*" >> "$trigger_log"
}

stop() {
	printf 'stop\n' >> "$service_log"
}

start() {
	printf 'start\n' >> "$service_log"
	start_service
}

cat > "$tmp/functions.sh" <<'EOF'
config_load() {
	[ "$1" = "qnatter" ] || return 1
}

config_foreach() {
	local callback="$1"
	local type="$2"
	[ "$type" = "instance" ] || return 0
	local s
	for s in $INSTANCES; do
		"$callback" "$s"
	done
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
		global:enabled) value="$GLOBAL_ENABLED" ;;
		global:hot_reload) value="$HOT_RELOAD" ;;
		wan_ct:enabled) value="1" ;;
		wan_ct:label) value="Telecom" ;;
		wan_ct:protocol) value="tcp" ;;
		wan_ct:network) value="wan" ;;
		wan_ct:bind_value) value="$BIND_VALUE" ;;
		wan_ct:forward_method) value="none" ;;
		wan_ct:auto_firewall) value="1" ;;
		wan_ct:cloudflare_enabled) value="1" ;;
		wan_ct:cloudflare_api_token) value="$CF_TOKEN" ;;
		wan_ct:cloudflare_zone_id) value="zone-selected" ;;
		wan_ct:cloudflare_record_id) value="record-selected" ;;
		wan_ct:route_slot) value="0" ;;
		wan_cm:enabled) value="1" ;;
		wan_cm:label) value="Mobile" ;;
		wan_cm:protocol) value="tcp" ;;
		wan_cm:network) value="wan_cmcc" ;;
		wan_cm:bind_value) value="pppoe-wan-cmcc" ;;
		wan_cm:forward_method) value="none" ;;
		wan_cm:auto_firewall) value="1" ;;
		wan_cm:cloudflare_enabled) value="0" ;;
		wan_cm:route_slot) value="1" ;;
	esac

	eval "$__var=\$value"
}

config_list_foreach() {
	return 0
}
EOF

cat > "$tmp/network.sh" <<'EOF'
network_get_device() {
	local __var="$1"
	local network="$2"
	local device=""

	case "$network" in
		wan) device="pppoe-wan" ;;
		wan_cmcc) device="pppoe-wan-cmcc" ;;
	esac

	[ -n "$device" ] || return 1
	eval "$__var=\$device"
}
EOF

cat > "$tmp/nft" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >> "$QNATTER_TEST_NFT_LOG"
EOF
chmod +x "$tmp/nft"

cat > "$tmp/ip" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >> "$QNATTER_TEST_IP_LOG"
case "$*" in
	"-4 -o addr show dev pppoe-wan scope global")
		printf '7: pppoe-wan inet %s/32 peer 100.64.0.1 scope global pppoe-wan\n' "$WAN_ADDR"
		;;
	"rule show")
		;;
esac
EOF
chmod +x "$tmp/ip"

QNATTER_FUNCTIONS_SH="$tmp/functions.sh"
QNATTER_NETWORK_SH="$tmp/network.sh"
QNATTER_COMMON_SH="$ROOT/qnatter/files/qnatter-common.sh"
QNATTER_QBITTORRENT_SH="$ROOT/qnatter/files/qnatter-qbittorrent.sh"
QNATTER_RUN_DIR="$tmp/run"
QNATTER_LOG_DIR="$tmp/log"
QNATTER_NFT_BIN="$tmp/nft"
QNATTER_IP_BIN="$tmp/ip"
QNATTER_TEST_NFT_LOG="$nft_log"
QNATTER_TEST_IP_LOG="$ip_log"
export QNATTER_FUNCTIONS_SH QNATTER_NETWORK_SH QNATTER_COMMON_SH QNATTER_QBITTORRENT_SH QNATTER_RUN_DIR QNATTER_LOG_DIR QNATTER_NFT_BIN QNATTER_IP_BIN QNATTER_TEST_NFT_LOG QNATTER_TEST_IP_LOG
INSTANCES="wan_ct"
export GLOBAL_ENABLED HOT_RELOAD BIND_VALUE CF_TOKEN INSTANCES WAN_ADDR

. "$ROOT/qnatter/files/qnatter.init"

start_service
[ -s "$tmp/run/wan_ct.runtime" ] || fail "start_service did not write runtime fingerprint"
grep -Fqx 'bind_ipv4=100.64.1.1' "$tmp/run/wan_ct.runtime" || fail "runtime fingerprint missing current bind IPv4"
grep -Fq "CLOUDFLARE_API_TOKEN='token-one'" "$tmp/run/wan_ct.env" || fail "initial notify env missing token"
grep -Fqx 'flush chain ip qnatter qnatter_dnat' "$nft_log" || fail "initial start did not flush stale DNAT rules"
grep -Fqx 'flush chain ip qnatter qnatter_snat' "$nft_log" || fail "initial start did not flush stale SNAT rules"
grep -Fqx 'flush chain ip qnatter qnatter_mark' "$nft_log" || fail "initial start did not flush stale mark rules"

# Test: same config but different interface address must produce a different
# runtime fingerprint so WAN redial reloads the instance.
mkdir -p "$tmp/addr-old" "$tmp/addr-new"
WAN_ADDR="100.64.1.1"
QNATTER_PREPARE_ONLY=1
QNATTER_RUNTIME_DIR="$tmp/addr-old"
QNATTER_RUNTIME_LIST="$tmp/addr-old/.runtime-instances"
QNATTER_ROUTE_SLOT=0
: > "$QNATTER_RUNTIME_LIST"
qnatter_start_instance wan_ct
WAN_ADDR="100.64.2.2"
QNATTER_RUNTIME_DIR="$tmp/addr-new"
QNATTER_RUNTIME_LIST="$tmp/addr-new/.runtime-instances"
QNATTER_ROUTE_SLOT=0
: > "$QNATTER_RUNTIME_LIST"
qnatter_start_instance wan_ct
unset QNATTER_PREPARE_ONLY QNATTER_RUNTIME_DIR QNATTER_RUNTIME_LIST QNATTER_ROUTE_SLOT QNATTER_ROUTE_MARK QNATTER_ROUTE_TABLE QNATTER_ROUTE_PRIORITY
if cmp -s "$tmp/addr-old/wan_ct.runtime" "$tmp/addr-new/wan_ct.runtime"; then
	fail "runtime fingerprint must change when bind IPv4 changes"
fi
grep -Fqx 'bind_ipv4=100.64.2.2' "$tmp/addr-new/wan_ct.runtime" || fail "updated runtime fingerprint missing new bind IPv4"

# Test: notify-only change (e.g. CF token) re-declares instances via procd, no flush
: > "$service_log"
: > "$procd_log"
: > "$nft_log"
CF_TOKEN="token-two"
reload_service

[ ! -s "$service_log" ] || fail "notify-only reload should not restart backend"
[ ! -s "$nft_log" ] || fail "notify-only reload must not flush nft runtime rules"
grep -Fq "CLOUDFLARE_API_TOKEN='token-two'" "$tmp/run/wan_ct.env" || fail "hot reload did not rewrite notify env"

# Test: runtime change re-declares instances, procd handles per-instance restart
: > "$service_log"
: > "$nft_log"
BIND_VALUE="pppoe-wan-new"
reload_service
[ ! -s "$service_log" ] || fail "runtime reload must not trigger full restart"
[ ! -s "$nft_log" ] || fail "runtime reload must not flush nft runtime rules"

# Test: HOT_RELOAD=0 same incremental behavior
: > "$service_log"
: > "$nft_log"
HOT_RELOAD=0
reload_service
[ ! -s "$service_log" ] || fail "reload must not trigger full restart even without hot_reload"
[ ! -s "$nft_log" ] || fail "reload must not flush nft runtime rules"

# Test: disabled service stops
: > "$service_log"
: > "$nft_log"
GLOBAL_ENABLED=0
reload_service
grep -Fqx 'stop' "$service_log" || fail "disabled service reload must stop backend"
if grep -Fqx 'start' "$service_log"; then
	fail "disabled service reload must not start backend"
fi
[ ! -s "$nft_log" ] || fail "disabled service reload must not flush nft runtime rules"

# Test: adding new instance re-declares, procd starts only the new one
: > "$service_log"
: > "$procd_log"
: > "$nft_log"
INSTANCES="wan_ct wan_cm"
GLOBAL_ENABLED=1
HOT_RELOAD=1
BIND_VALUE="pppoe-wan"
reload_service
[ ! -s "$service_log" ] || fail "adding new instance must not restart existing instances"
[ ! -s "$nft_log" ] || fail "adding new instance must not flush nft runtime rules"

# Test: reorder-only reload
: > "$service_log"
: > "$procd_log"
: > "$nft_log"
INSTANCES="wan_cm wan_ct"
reload_service
[ ! -s "$service_log" ] || fail "reorder-only reload should not restart backend"
[ ! -s "$nft_log" ] || fail "reorder-only reload must not flush nft runtime rules"

printf 'qnatter reload checks passed\n'
