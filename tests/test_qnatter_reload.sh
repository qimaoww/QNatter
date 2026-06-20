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
	"$callback" wan_ct
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
export GLOBAL_ENABLED HOT_RELOAD BIND_VALUE CF_TOKEN

. "$ROOT/qnatter/files/qnatter.init"

start_service
[ -s "$tmp/run/wan_ct.runtime" ] || fail "start_service did not write runtime fingerprint"
grep -Fq "CLOUDFLARE_API_TOKEN='token-one'" "$tmp/run/wan_ct.env" || fail "initial notify env missing token"
grep -Fqx 'flush chain ip qnatter qnatter_dnat' "$nft_log" || fail "initial start did not flush stale DNAT rules"
grep -Fqx 'flush chain ip qnatter qnatter_snat' "$nft_log" || fail "initial start did not flush stale SNAT rules"
grep -Fqx 'flush chain ip qnatter qnatter_mark' "$nft_log" || fail "initial start did not flush stale mark rules"

: > "$service_log"
: > "$procd_log"
: > "$nft_log"
CF_TOKEN="token-two"
reload_service

[ ! -s "$service_log" ] || fail "notify-only reload should not restart backend"
[ ! -s "$procd_log" ] || fail "notify-only reload should not reopen procd instance"
[ ! -s "$nft_log" ] || fail "notify-only reload must not flush nft runtime rules"
grep -Fq "CLOUDFLARE_API_TOKEN='token-two'" "$tmp/run/wan_ct.env" || fail "hot reload did not rewrite notify env"

: > "$service_log"
: > "$nft_log"
BIND_VALUE="pppoe-wan-new"
reload_service
grep -Fqx 'stop' "$service_log" || fail "runtime reload must stop backend"
grep -Fqx 'start' "$service_log" || fail "runtime reload must start backend"
grep -Fqx 'flush chain ip qnatter qnatter_dnat' "$nft_log" || fail "runtime reload did not flush stale DNAT rules"
grep -Fqx 'flush chain ip qnatter qnatter_snat' "$nft_log" || fail "runtime reload did not flush stale SNAT rules"
grep -Fqx 'flush chain ip qnatter qnatter_mark' "$nft_log" || fail "runtime reload did not flush stale mark rules"

: > "$service_log"
: > "$nft_log"
HOT_RELOAD=0
reload_service
grep -Fqx 'stop' "$service_log" || fail "disabled hot reload must stop backend"
grep -Fqx 'start' "$service_log" || fail "disabled hot reload must start backend"
grep -Fqx 'flush chain ip qnatter qnatter_dnat' "$nft_log" || fail "disabled hot reload start did not flush stale DNAT rules"
grep -Fqx 'flush chain ip qnatter qnatter_snat' "$nft_log" || fail "disabled hot reload start did not flush stale SNAT rules"
grep -Fqx 'flush chain ip qnatter qnatter_mark' "$nft_log" || fail "disabled hot reload start did not flush stale mark rules"

: > "$service_log"
: > "$nft_log"
GLOBAL_ENABLED=0
reload_service
grep -Fqx 'stop' "$service_log" || fail "disabled service reload must stop backend"
if grep -Fqx 'start' "$service_log"; then
	fail "disabled service reload must not start backend"
fi
[ ! -s "$nft_log" ] || fail "disabled service reload must not flush nft runtime rules"

printf 'qnatter reload checks passed\n'
