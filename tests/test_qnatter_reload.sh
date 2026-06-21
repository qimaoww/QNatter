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
current_instance=""
GLOBAL_ENABLED=1
HOT_RELOAD=1
BIND_VALUE="pppoe-wan"
CF_TOKEN="token-one"

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
# Not needed by qnatter.init in tests.
EOF

cat > "$tmp/nft" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >> "$QNATTER_TEST_NFT_LOG"
EOF
chmod +x "$tmp/nft"

QNATTER_FUNCTIONS_SH="$tmp/functions.sh"
QNATTER_NETWORK_SH="$tmp/network.sh"
QNATTER_COMMON_SH="$ROOT/qnatter/files/qnatter-common.sh"
QNATTER_QBITTORRENT_SH="$ROOT/qnatter/files/qnatter-qbittorrent.sh"
QNATTER_RUN_DIR="$tmp/run"
QNATTER_LOG_DIR="$tmp/log"
QNATTER_NFT_BIN="$tmp/nft"
QNATTER_TEST_NFT_LOG="$nft_log"
export QNATTER_FUNCTIONS_SH QNATTER_NETWORK_SH QNATTER_COMMON_SH QNATTER_QBITTORRENT_SH QNATTER_RUN_DIR QNATTER_LOG_DIR QNATTER_NFT_BIN QNATTER_TEST_NFT_LOG
INSTANCES="wan_ct"
export GLOBAL_ENABLED HOT_RELOAD BIND_VALUE CF_TOKEN INSTANCES

. "$ROOT/qnatter/files/qnatter.init"

start_service
[ -s "$tmp/run/wan_ct.runtime" ] || fail "start_service did not write runtime fingerprint"
grep -Fq "CLOUDFLARE_API_TOKEN='token-one'" "$tmp/run/wan_ct.env" || fail "initial notify env missing token"
grep -Fqx 'flush chain ip qnatter qnatter_dnat' "$nft_log" || fail "initial start did not flush stale DNAT rules"
grep -Fqx 'flush chain ip qnatter qnatter_snat' "$nft_log" || fail "initial start did not flush stale SNAT rules"
grep -Fqx 'flush chain ip qnatter qnatter_mark' "$nft_log" || fail "initial start did not flush stale mark rules"

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
