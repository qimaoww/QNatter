#!/bin/sh

set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	exit 1
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

config_file="$tmp/qnatter"
default_file="$tmp/qnatter.default"
uci_bin="$tmp/uci"
uci_calls="$tmp/uci-calls.txt"

printf 'default-config\n' > "$default_file"

QNATTER_UCI_CONFIG="$config_file" \
QNATTER_UCI_DEFAULT="$default_file" \
	sh "$ROOT/qnatter/files/qnatter.uci-default"

[ "$(cat "$config_file")" = "default-config" ] || fail "uci-defaults did not initialize missing config"

printf 'user-config\n' > "$config_file"

QNATTER_UCI_CONFIG="$config_file" \
QNATTER_UCI_DEFAULT="$default_file" \
	sh "$ROOT/qnatter/files/qnatter.uci-default"

[ "$(cat "$config_file")" = "user-config" ] || fail "uci-defaults overwrote existing user config"

cat > "$uci_bin" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >> "$uci_calls"
case "\$*" in
	"-q get qnatter.global.hot_reload")
		exit 1
		;;
	"-q get qnatter.wan_ct.route_slot")
		exit 1
		;;
	"-q get qnatter.wan_cm.route_slot")
		printf '%s\n' '0'
		exit 0
		;;
	"-q get qnatter.wan_qb.route_slot")
		printf '%s\n' 'bad'
		exit 0
		;;
	"-q show qnatter")
		printf '%s\n' 'qnatter.global=global' 'qnatter.wan_ct=instance' 'qnatter.wan_cm=instance' 'qnatter.wan_qb=instance'
		exit 0
		;;
	"set qnatter.global.hot_reload=1"|"-q delete qnatter.wan_ct.label"|"-q delete qnatter.wan_cm.label"|"-q delete qnatter.wan_qb.label"|"set qnatter.wan_ct.route_slot=0"|"set qnatter.wan_cm.route_slot=1"|"set qnatter.wan_qb.route_slot=2"|"commit qnatter")
		exit 0
		;;
esac
exit 1
EOF
chmod 0755 "$uci_bin"
: > "$uci_calls"

QNATTER_UCI_CONFIG="$config_file" \
QNATTER_UCI_DEFAULT="$default_file" \
QNATTER_UCI_BIN="$uci_bin" \
	sh "$ROOT/qnatter/files/qnatter.uci-default"

for want in \
	'-q get qnatter.global.hot_reload' \
	'set qnatter.global.hot_reload=1' \
	'-q show qnatter' \
	'-q delete qnatter.wan_ct.label' \
	'-q delete qnatter.wan_cm.label' \
	'-q delete qnatter.wan_qb.label' \
	'-q get qnatter.wan_ct.route_slot' \
	'set qnatter.wan_ct.route_slot=0' \
	'-q get qnatter.wan_cm.route_slot' \
	'set qnatter.wan_cm.route_slot=1' \
	'-q get qnatter.wan_qb.route_slot' \
	'set qnatter.wan_qb.route_slot=2' \
	'commit qnatter'
do
	grep -Fqx -- "$want" "$uci_calls" || fail "uci-defaults migration did not run: $want"
done

rm -f "$config_file" "$default_file"

QNATTER_UCI_CONFIG="$config_file" \
QNATTER_UCI_DEFAULT="$default_file" \
	sh "$ROOT/qnatter/files/qnatter.uci-default"

[ ! -e "$config_file" ] || fail "uci-defaults created config when default was missing"

printf 'qnatter uci-default checks passed\n'
