#!/bin/sh

set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	exit 1
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

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
		wan_ct:enabled) value="1" ;;
		wan_ct:network) value="wan" ;;
		wan_ct:bind_value) value="pppoe-wan" ;;
		wan_cm:enabled) value="1" ;;
		wan_cm:network) value="wan2" ;;
		wan_cm:bind_value) value="eth1.2" ;;
		wan_auto:enabled) value="1" ;;
		wan_auto:network) value="wan3" ;;
		wan_auto:bind_value) value="" ;;
		disabled_lan:enabled) value="0" ;;
		disabled_lan:network) value="lan" ;;
		disabled_lan:bind_value) value="br-lan" ;;
	esac

	eval "$__var=\$value"
}
EOF

init_log="$tmp/init.log"
init_bin="$tmp/qnatter-init"
cat > "$init_bin" <<EOF
#!/bin/sh
printf '%s\n' "\$1" >> "$init_log"
case "\$1" in
	enabled) exit 0 ;;
	reload) exit 0 ;;
	*) exit 1 ;;
esac
EOF
chmod 0755 "$init_bin"

run_hotplug() {
	: > "$init_log"
	ACTION="$1" \
	INTERFACE="$2" \
	DEVICE="$3" \
	QNATTER_FUNCTIONS_SH="$tmp/functions.sh" \
	QNATTER_INIT_BIN="$init_bin" \
		sh "$ROOT/qnatter/files/qnatter.hotplug"
}

run_hotplug ifup wan pppoe-wan
grep -Fqx enabled "$init_log" || fail "ifup wan should check service enabled"
grep -Fqx reload "$init_log" || fail "ifup wan should reload QNatter"

run_hotplug ifup wan eth0
if [ -s "$init_log" ]; then
	fail "ifup wan with non-matching device must not reload bound QNatter instance"
fi

run_hotplug ifupdate other eth1.2
grep -Fqx reload "$init_log" || fail "ifupdate matching bind_value should reload QNatter"

run_hotplug ifup wan3 eth9
grep -Fqx reload "$init_log" || fail "instance without bind_value should reload by network"

run_hotplug ifup lan br-lan
if [ -s "$init_log" ]; then
	fail "disabled matching instance must not reload QNatter"
fi

run_hotplug ifdown wan pppoe-wan
if [ -s "$init_log" ]; then
	fail "ifdown must not reload QNatter"
fi

printf 'qnatter hotplug checks passed\n'
