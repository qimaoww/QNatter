#!/bin/sh

set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	exit 1
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

status_file="$tmp/wan_ct.json"
args_file="$tmp/user-args.txt"
user_script="$tmp/user-notify.sh"
env_file="$tmp/notify.env"

cat > "$user_script" <<EOF
#!/bin/sh
printf '%s\n' "\$@" > "$args_file"
EOF
chmod 0755 "$user_script"

cat > "$env_file" <<EOF
NATTER_INSTANCE='wan_ct'
NATTER_STATUS_FILE='$status_file'
NATTER_USER_NOTIFY='$user_script'
QBITTORRENT_ENABLED='0'
EOF

NATTER_COMMON_SH="$ROOT/natter/files/natter-common.sh" \
NATTER_QBITTORRENT_SH="$ROOT/natter/files/natter-qbittorrent.sh" \
NATTER_NOTIFY_ENV="$env_file" \
	"$ROOT/natter/files/natter-notify" tcp 10.10.10.10 51413 203.0.113.10 62000

[ -s "$status_file" ] || fail "status file was not written"

for want in \
	'"instance":"wan_ct"' \
	'"protocol":"tcp"' \
	'"inner_ip":"10.10.10.10"' \
	'"inner_port":51413' \
	'"outer_ip":"203.0.113.10"' \
	'"outer_port":62000' \
	'"message":"mapped"'
do
	grep -Fq "$want" "$status_file" || fail "status file is missing $want"
done

expected_args="$tmp/expected-args.txt"
cat > "$expected_args" <<'EOF'
tcp
10.10.10.10
51413
203.0.113.10
62000
EOF

cmp -s "$expected_args" "$args_file" || fail "user notify args did not match mapping"

printf 'natter notify checks passed\n'
