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
uci_bin="$tmp/uci"
uci_calls="$tmp/uci-calls.txt"
firewall_bin="$tmp/firewall"
firewall_calls="$tmp/firewall-calls.txt"

cat > "$user_script" <<EOF
#!/bin/sh
printf '%s\n' "\$@" > "$args_file"
EOF
chmod 0755 "$user_script"

cat > "$uci_bin" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >> "$uci_calls"
exit 0
EOF
chmod 0755 "$uci_bin"

cat > "$firewall_bin" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >> "$firewall_calls"
exit 0
EOF
chmod 0755 "$firewall_bin"

cat > "$env_file" <<EOF
NATTER_INSTANCE='wan_ct'
NATTER_STATUS_FILE='$status_file'
NATTER_USER_NOTIFY='$user_script'
NATTER_AUTO_FIREWALL='1'
NATTER_FIREWALL_SECTION='natter_wan_ct'
NATTER_FIREWALL_NAME='Natter WAN CT'
NATTER_FIREWALL_SRC='wan'
QBITTORRENT_ENABLED='0'
EOF

NATTER_COMMON_SH="$ROOT/natter/files/natter-common.sh" \
NATTER_QBITTORRENT_SH="$ROOT/natter/files/natter-qbittorrent.sh" \
NATTER_UCI_BIN="$uci_bin" \
NATTER_FIREWALL_INIT="$firewall_bin" \
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

for want in \
	'-q delete firewall.natter_wan_ct' \
	'set firewall.natter_wan_ct=rule' \
	'set firewall.natter_wan_ct.name=Natter WAN CT' \
	'set firewall.natter_wan_ct.src=wan' \
	'set firewall.natter_wan_ct.proto=tcp' \
	'set firewall.natter_wan_ct.dest_port=51413' \
	'set firewall.natter_wan_ct.target=ACCEPT' \
	'commit firewall'
do
	grep -Fqx -- "$want" "$uci_calls" || fail "auto firewall did not run uci command: $want"
done
grep -Fqx 'reload' "$firewall_calls" || fail "auto firewall did not reload firewall"

qb_status_file="$tmp/wan_cm.json"
qb_args_file="$tmp/qb-user-args.txt"
qb_user_script="$tmp/qb-user-notify.sh"
qb_env_file="$tmp/qb-notify.env"
logger_bin="$tmp/logger"
logger_file="$tmp/logger.txt"

cat > "$qb_user_script" <<EOF
#!/bin/sh
printf '%s\n' "\$@" > "$qb_args_file"
EOF
chmod 0755 "$qb_user_script"

cat > "$logger_bin" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >> "$logger_file"
EOF
chmod 0755 "$logger_bin"

cat > "$qb_env_file" <<EOF
NATTER_INSTANCE='wan_cm'
NATTER_STATUS_FILE='$qb_status_file'
NATTER_USER_NOTIFY='$qb_user_script'
QBITTORRENT_ENABLED='1'
QBITTORRENT_URL=''
QBITTORRENT_USERNAME=''
EOF

NATTER_COMMON_SH="$ROOT/natter/files/natter-common.sh" \
NATTER_QBITTORRENT_SH="$ROOT/natter/files/natter-qbittorrent.sh" \
NATTER_LOGGER_BIN="$logger_bin" \
NATTER_NOTIFY_ENV="$qb_env_file" \
	"$ROOT/natter/files/natter-notify" udp 10.10.10.20 51414 198.51.100.20 62001

grep -Fq '"message":"qBittorrent URL or username is empty"' "$qb_status_file" || \
	fail "qBittorrent missing config message was not written to status"
grep -Fq 'qBittorrent is enabled but URL or username is empty for wan_cm' "$logger_file" || \
	fail "qBittorrent missing config warning was not logged"
cmp -s "$expected_args" "$args_file" || fail "original user notify args changed"

cat > "$expected_args" <<'EOF'
udp
10.10.10.20
51414
198.51.100.20
62001
EOF
cmp -s "$expected_args" "$qb_args_file" || fail "qBittorrent error path did not call user notify"

qb_login_fail_status_file="$tmp/wan_login_fail.json"
qb_login_fail_env_file="$tmp/qb-login-fail.env"
curl_login_fail_bin="$tmp/curl-login-fail"
curl_login_fail_calls="$tmp/curl-login-fail-calls.txt"

cat > "$curl_login_fail_bin" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >> "$curl_login_fail_calls"
case "\$*" in
	*/api/v2/auth/login*)
		printf 'Fails.'
		exit 0
		;;
	*/api/v2/app/setPreferences*)
		exit 0
		;;
esac
exit 1
EOF
chmod 0755 "$curl_login_fail_bin"

cat > "$qb_login_fail_env_file" <<EOF
NATTER_INSTANCE='wan_login_fail'
NATTER_STATUS_FILE='$qb_login_fail_status_file'
QBITTORRENT_ENABLED='1'
QBITTORRENT_URL='http://127.0.0.1:9'
QBITTORRENT_USERNAME='admin'
QBITTORRENT_PASSWORD='wrong'
QBITTORRENT_TARGET_PORT='0'
QBITTORRENT_TIMEOUT='1'
EOF

NATTER_COMMON_SH="$ROOT/natter/files/natter-common.sh" \
NATTER_QBITTORRENT_SH="$ROOT/natter/files/natter-qbittorrent.sh" \
NATTER_LOGGER_BIN="$logger_bin" \
NATTER_CURL_BIN="$curl_login_fail_bin" \
NATTER_NOTIFY_ENV="$qb_login_fail_env_file" \
	"$ROOT/natter/files/natter-notify" tcp 10.10.10.25 51416 203.0.113.25 62003

grep -Fq '"message":"qBittorrent login failed"' "$qb_login_fail_status_file" || \
	fail "qBittorrent login failure response was not written to status"
if grep -Fq '/api/v2/app/setPreferences' "$curl_login_fail_calls"; then
	fail "qBittorrent preferences API must not be called after login response Fails."
fi

qb_ok_status_file="$tmp/wan_ok.json"
qb_ok_args_file="$tmp/qb-ok-user-args.txt"
qb_ok_user_script="$tmp/qb-ok-user-notify.sh"
qb_ok_env_file="$tmp/qb-ok-notify.env"
curl_bin="$tmp/curl"
curl_calls="$tmp/curl-calls.txt"

cat > "$qb_ok_user_script" <<EOF
#!/bin/sh
printf '%s\n' "\$@" > "$qb_ok_args_file"
EOF
chmod 0755 "$qb_ok_user_script"

cat > "$curl_bin" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >> "$curl_calls"
case "\$*" in
	*/api/v2/auth/login*) printf 'Ok.' ;;
esac
exit 0
EOF
chmod 0755 "$curl_bin"

cat > "$qb_ok_env_file" <<EOF
NATTER_INSTANCE='wan_ok'
NATTER_STATUS_FILE='$qb_ok_status_file'
NATTER_USER_NOTIFY='$qb_ok_user_script'
QBITTORRENT_ENABLED='1'
QBITTORRENT_URL='http://127.0.0.1:9/'
QBITTORRENT_USERNAME='admin'
QBITTORRENT_PASSWORD='secret'
QBITTORRENT_TARGET_PORT='0'
QBITTORRENT_TIMEOUT='1'
EOF

NATTER_COMMON_SH="$ROOT/natter/files/natter-common.sh" \
NATTER_QBITTORRENT_SH="$ROOT/natter/files/natter-qbittorrent.sh" \
NATTER_LOGGER_BIN="$logger_bin" \
NATTER_CURL_BIN="$curl_bin" \
NATTER_NOTIFY_ENV="$qb_ok_env_file" \
	"$ROOT/natter/files/natter-notify" tcp 10.10.10.30 51415 203.0.113.30 62002

grep -Fq '/api/v2/auth/login' "$curl_calls" || fail "qBittorrent login API was not called"
grep -Fq 'username=admin' "$curl_calls" || fail "qBittorrent username was not sent"
grep -Fq 'password=secret' "$curl_calls" || fail "qBittorrent password was not sent"
grep -Fq '/api/v2/app/setPreferences' "$curl_calls" || fail "qBittorrent preferences API was not called"
grep -Fq 'json={"listen_port":62002}' "$curl_calls" || fail "qBittorrent listen_port should use outer port"
grep -Fq '"message":"qBittorrent listen_port set to 62002"' "$qb_ok_status_file" || \
	fail "qBittorrent success message was not written to status"

cat > "$expected_args" <<'EOF'
tcp
10.10.10.30
51415
203.0.113.30
62002
EOF
cmp -s "$expected_args" "$qb_ok_args_file" || fail "qBittorrent success path did not call user notify"

qb_trap_status_file="$tmp/wan_trap.json"
qb_trap_env_file="$tmp/qb-trap-notify.env"
curl_trap_bin="$tmp/curl-trap"
trap_cookie="/tmp/natter-qbittorrent-wan_trap.cookie"
rm -f "$trap_cookie"

cat > "$curl_trap_bin" <<EOF
#!/bin/sh
case "\$*" in
	*/api/v2/auth/login*)
		while [ "\$#" -gt 0 ]; do
			if [ "\$1" = "-c" ]; then
				shift
				printf 'cookie\n' > "\$1"
				break
			fi
			shift
		done
		printf 'Ok.'
		exit 0
		;;
	*/api/v2/app/setPreferences*)
		kill -TERM "\$PPID"
		sleep 1
		exit 1
		;;
esac
exit 1
EOF
chmod 0755 "$curl_trap_bin"

cat > "$qb_trap_env_file" <<EOF
NATTER_INSTANCE='wan_trap'
NATTER_STATUS_FILE='$qb_trap_status_file'
QBITTORRENT_ENABLED='1'
QBITTORRENT_URL='http://127.0.0.1:9/'
QBITTORRENT_USERNAME='admin'
QBITTORRENT_PASSWORD='secret'
QBITTORRENT_TARGET_PORT='0'
QBITTORRENT_TIMEOUT='1'
EOF

NATTER_COMMON_SH="$ROOT/natter/files/natter-common.sh" \
NATTER_QBITTORRENT_SH="$ROOT/natter/files/natter-qbittorrent.sh" \
NATTER_LOGGER_BIN="$logger_bin" \
NATTER_CURL_BIN="$curl_trap_bin" \
NATTER_NOTIFY_ENV="$qb_trap_env_file" \
	"$ROOT/natter/files/natter-notify" tcp 10.10.10.31 51415 203.0.113.31 62004 || true

[ ! -e "$trap_cookie" ] || fail "qBittorrent cookie must be removed when notify is interrupted"

printf 'natter notify checks passed\n'
