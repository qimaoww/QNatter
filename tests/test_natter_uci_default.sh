#!/bin/sh

set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	exit 1
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

config_file="$tmp/natter"
default_file="$tmp/natter.default"
uci_bin="$tmp/uci"
uci_calls="$tmp/uci-calls.txt"

printf 'default-config\n' > "$default_file"

NATTER_UCI_CONFIG="$config_file" \
NATTER_UCI_DEFAULT="$default_file" \
	sh "$ROOT/natter/files/natter.uci-default"

[ "$(cat "$config_file")" = "default-config" ] || fail "uci-defaults did not initialize missing config"

printf 'user-config\n' > "$config_file"

NATTER_UCI_CONFIG="$config_file" \
NATTER_UCI_DEFAULT="$default_file" \
	sh "$ROOT/natter/files/natter.uci-default"

[ "$(cat "$config_file")" = "user-config" ] || fail "uci-defaults overwrote existing user config"

cat > "$uci_bin" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >> "$uci_calls"
case "\$*" in
	"-q get natter.global.hot_reload")
		exit 1
		;;
	"-q get natter.wan_ct.route_slot")
		exit 1
		;;
	"-q get natter.wan_cm.route_slot")
		printf '%s\n' '0'
		exit 0
		;;
	"-q get natter.wan_qb.route_slot")
		printf '%s\n' 'bad'
		exit 0
		;;
	"-q show natter")
		printf '%s\n' 'natter.global=global' 'natter.wan_ct=instance' 'natter.wan_cm=instance' 'natter.wan_qb=instance'
		exit 0
		;;
	"set natter.global.hot_reload=1"|"-q delete natter.wan_ct.label"|"-q delete natter.wan_cm.label"|"-q delete natter.wan_qb.label"|"set natter.wan_ct.route_slot=0"|"set natter.wan_cm.route_slot=1"|"set natter.wan_qb.route_slot=2"|"commit natter")
		exit 0
		;;
esac
exit 1
EOF
chmod 0755 "$uci_bin"
: > "$uci_calls"

NATTER_UCI_CONFIG="$config_file" \
NATTER_UCI_DEFAULT="$default_file" \
NATTER_UCI_BIN="$uci_bin" \
	sh "$ROOT/natter/files/natter.uci-default"

for want in \
	'-q get natter.global.hot_reload' \
	'set natter.global.hot_reload=1' \
	'-q show natter' \
	'-q delete natter.wan_ct.label' \
	'-q delete natter.wan_cm.label' \
	'-q delete natter.wan_qb.label' \
	'-q get natter.wan_ct.route_slot' \
	'set natter.wan_ct.route_slot=0' \
	'-q get natter.wan_cm.route_slot' \
	'set natter.wan_cm.route_slot=1' \
	'-q get natter.wan_qb.route_slot' \
	'set natter.wan_qb.route_slot=2' \
	'commit natter'
do
	grep -Fqx -- "$want" "$uci_calls" || fail "uci-defaults migration did not run: $want"
done

rm -f "$config_file" "$default_file"

NATTER_UCI_CONFIG="$config_file" \
NATTER_UCI_DEFAULT="$default_file" \
	sh "$ROOT/natter/files/natter.uci-default"

[ ! -e "$config_file" ] || fail "uci-defaults created config when default was missing"

printf 'natter uci-default checks passed\n'
