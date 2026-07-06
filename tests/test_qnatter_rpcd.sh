#!/bin/sh

set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	exit 1
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

status_bin="$tmp/qnatter-status"
log_bin="$tmp/qnatter-log"
log_calls="$tmp/log-calls.txt"
uci_bin="$tmp/uci"
mv_bin="$tmp/mv"
init_bin="$tmp/qnatter-init"
curl_bin="$tmp/curl-cloudflare"
jsonfilter_bin="$tmp/jsonfilter"
ubus_bin="$tmp/ubus"
curl_calls="$tmp/curl-calls.txt"
uci_calls="$tmp/uci-calls.txt"
mv_calls="$tmp/mv-calls.txt"
init_calls="$tmp/init-calls.txt"
ubus_calls="$tmp/ubus-calls.txt"
run_dir="$tmp/run"
log_dir="$tmp/logs"
mkdir -p "$run_dir" "$log_dir"

cat > "$status_bin" <<'EOF'
#!/bin/sh
printf '{"instances":[{"name":"wan_ct","running":true}]}\n'
EOF
chmod 0755 "$status_bin"

cat > "$log_bin" <<EOF
#!/bin/sh
printf '%s %s %s\n' "\$1" "\$2" "\$3" >> "$log_calls"
case "\$1" in
read)
	printf 'alpha "quoted"\n'
	printf 'beta \\\\ slash\n'
	printf 'gamma\tcarriage\rreturn\n'
	;;
clear)
	printf '{"ok":true}\n'
	;;
esac
EOF
chmod 0755 "$log_bin"

cat > "$uci_bin" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >> "$uci_calls"
quiet=0
[ "\$1" = "-q" ] && quiet=1 && shift
case "\$1" in
	get)
		case "\$2" in
			qnatter.wan_ct) printf 'instance\n' ;;
			qnatter.wan_ct.enabled) printf '0\n' ;;
			qnatter.wan_new) exit 1 ;;
			qnatter.bad-name) exit 1 ;;
			qnatter.wan_ct.cloudflare_api_token) printf 'cf-token\n' ;;
			qnatter.wan_ct.cloudflare_zone_id) printf 'zone1\n' ;;
			*) exit 1 ;;
		esac
		;;
	rename|delete|commit|set)
		exit 0
		;;
	*)
		exit 1
		;;
esac
EOF
chmod 0755 "$uci_bin"

cat > "$mv_bin" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >> "$mv_calls"
exit 0
EOF
chmod 0755 "$mv_bin"

cat > "$init_bin" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >> "$init_calls"
exit 0
EOF
chmod 0755 "$init_bin"

cat > "$curl_bin" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >> "$curl_calls"
case "\$*" in
	*'hooks.example/test'*)
		printf 'manual-ok 62000\n'
		;;
	*'/zones/zone1/dns_records?type=SRV&per_page=100'*)
		cat <<'JSON'
{"result":[{"id":"record1","name":"_qb._tcp.example.com","data":{"target":"qb.example.com","port":51413}},{"id":"record2","name":"_mc._tcp.example.com","data":{"target":"mc.example.com","port":25565}}]}
JSON
		;;
	*'/zones?per_page=100'*)
		cat <<'JSON'
{"result":[{"id":"zone1","name":"example.com"},{"id":"zone2","name":"example.net"}]}
JSON
		;;
	*) exit 22 ;;
esac
EOF
chmod 0755 "$curl_bin"

cat > "$jsonfilter_bin" <<'EOF'
#!/bin/sh
expr=""
while [ "$#" -gt 0 ]; do
	case "$1" in
		-e)
			shift
			expr="$1"
			;;
	esac
	shift
done
input="$(cat)"
case "$input:$expr" in
	*'hooks.example/test'*':@.section') printf 'wan_ct\n' ;;
	*'hooks.example/test'*':@.url') printf 'https://hooks.example/test\n' ;;
	*'hooks.example/test'*':@.method') printf 'PUT\n' ;;
	*'hooks.example/test'*':@.headers') printf 'Content-Type: text/plain\nX-Test: #{port}\n' ;;
	*'hooks.example/test'*':@.body') printf 'port=#{port}\n' ;;
	*'hooks.example/test'*':@.success') printf 'manual-ok #{port}\n' ;;
	*'hooks.example/test'*':@.disable_success_check') printf '0\n' ;;
	*'hooks.example/test'*':@.timeout') printf '4\n' ;;
	*'hooks.example/test'*':@.skip_unchanged') printf '1\n' ;;
	*'example.net'*':@.result[*].id') printf 'zone1\nzone2\n' ;;
	*'example.net'*':@.result[*].name') printf 'example.com\nexample.net\n' ;;
	*'record2'*':@.result[*].id') printf 'record1\nrecord2\n' ;;
	*'record2'*':@.result[*].name') printf '_qb._tcp.example.com\n_mc._tcp.example.com\n' ;;
	*'record2'*':@.result[*].data.target') printf 'qb.example.com\nmc.example.com\n' ;;
	*'record2'*':@.result[*].data.port') printf '51413\n25565\n' ;;
	*) exit 1 ;;
esac
EOF
chmod 0755 "$jsonfilter_bin"

cat > "$ubus_bin" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >> "$ubus_calls"
exit 0
EOF
chmod 0755 "$ubus_bin"

rpcd="$ROOT/luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter"

list_output="$("$rpcd" list)"
printf '%s' "$list_output" | grep -Fq '"cloudflare_zones":{"section":"String","token":"String"}' || \
	fail "rpc list is missing cloudflare_zones signature: $list_output"
printf '%s' "$list_output" | grep -Fq '"cloudflare_srv_records":{"section":"String","zone_id":"String","token":"String"}' || \
	fail "rpc list is missing cloudflare_srv_records signature: $list_output"
printf '%s' "$list_output" | grep -Fq '"rename_instance":{"old":"String","new":"String"}' || \
	fail "rpc list is missing rename_instance signature: $list_output"
printf '%s' "$list_output" | grep -Fq '"toggle_instance":{"instance":"String"}' || \
	fail "rpc list is missing toggle_instance signature: $list_output"
printf '%s' "$list_output" | grep -Fq '"completion_webhook_test":{"section":"String","url":"String","method":"String","headers":"String","body":"String","success":"String","disable_success_check":"String","skip_unchanged":"String","timeout":"Integer"}' || \
	fail "rpc list is missing completion_webhook_test signature: $list_output"
printf '%s' "$list_output" | grep -Fq '"completion_script_test":{"section":"String","script":"String"}' || \
	fail "rpc list is missing completion_script_test signature: $list_output"

status_output="$(
	printf '{}\n' | QNATTER_STATUS_BIN="$status_bin" QNATTER_LOG_BIN="$log_bin" "$rpcd" call status
)"
[ "$status_output" = '{"instances":[{"name":"wan_ct","running":true}]}' ] || fail "status output = $status_output"

log_output="$(
	printf '{"instance":"wan_ct","lines":25}\n' | QNATTER_STATUS_BIN="$status_bin" QNATTER_LOG_BIN="$log_bin" "$rpcd" call log
)"
expected_log='{"log":"alpha \"quoted\"\nbeta \\ slash\ngamma\tcarriage\rreturn"}'
[ "$log_output" = "$expected_log" ] || fail "log output was not escaped correctly: $log_output"
grep -Fqx 'read wan_ct 25' "$log_calls" || fail "log helper was not called with requested instance and lines"

clear_output="$(
	printf '{"instance":"wan_ct"}\n' | QNATTER_STATUS_BIN="$status_bin" QNATTER_LOG_BIN="$log_bin" "$rpcd" call clear_log
)"
[ "$clear_output" = '{"ok":true}' ] || fail "clear output = $clear_output"
grep -Fqx 'clear wan_ct 200' "$log_calls" || fail "clear helper was not called with requested instance"

zones_output="$(
	printf '{"section":"wan_ct"}\n' | QNATTER_UCI_BIN="$uci_bin" QNATTER_CURL_BIN="$curl_bin" QNATTER_JSONFILTER_BIN="$jsonfilter_bin" "$rpcd" call cloudflare_zones
)"
[ "$zones_output" = '{"zones":[{"id":"zone1","name":"example.com"},{"id":"zone2","name":"example.net"}]}' ] || \
	fail "Cloudflare zones output = $zones_output"

records_output="$(
	printf '{"section":"wan_ct","zone_id":"zone1"}\n' | QNATTER_UCI_BIN="$uci_bin" QNATTER_CURL_BIN="$curl_bin" QNATTER_JSONFILTER_BIN="$jsonfilter_bin" "$rpcd" call cloudflare_srv_records
)"
[ "$records_output" = '{"records":[{"id":"record1","name":"_qb._tcp.example.com","target":"qb.example.com","port":51413},{"id":"record2","name":"_mc._tcp.example.com","target":"mc.example.com","port":25565}]}' ] || \
	fail "Cloudflare SRV records output = $records_output"
grep -Fq 'Authorization: Bearer cf-token' "$curl_calls" || fail "Cloudflare RPC did not send bearer token"
grep -Fq 'zones/zone1/dns_records?type=SRV&per_page=100' "$curl_calls" || fail "Cloudflare RPC did not request SRV records"

unsaved_zones_output="$(
	printf '{"section":"unsaved","token":"input-token"}\n' | QNATTER_UCI_BIN="$uci_bin" QNATTER_CURL_BIN="$curl_bin" QNATTER_JSONFILTER_BIN="$jsonfilter_bin" "$rpcd" call cloudflare_zones
)"
[ "$unsaved_zones_output" = '{"zones":[{"id":"zone1","name":"example.com"},{"id":"zone2","name":"example.net"}]}' ] || \
	fail "Cloudflare zones output must use unsaved input token: $unsaved_zones_output"
grep -Fq 'Authorization: Bearer input-token' "$curl_calls" || fail "Cloudflare RPC did not send unsaved input token"

webhook_test_output="$(
	printf '{"section":"wan_ct","url":"https://hooks.example/test","method":"PUT","headers":"Content-Type: text/plain\nX-Test: #{port}","body":"port=#{port}","success":"manual-ok #{port}","disable_success_check":"0","skip_unchanged":"1","timeout":4}\n' | \
		QNATTER_CURL_BIN="$curl_bin" \
		QNATTER_JSONFILTER_BIN="$jsonfilter_bin" \
		"$rpcd" call completion_webhook_test
)"
[ "$webhook_test_output" = '{"ok":true,"response":"manual-ok 62000"}' ] || fail "webhook test output = $webhook_test_output"
grep -Fq -- '-X PUT' "$curl_calls" || fail "webhook test did not use requested method"
grep -Fq -- 'Content-Type: text/plain' "$curl_calls" || fail "webhook test did not send requested header"
grep -Fq -- 'X-Test: 62000' "$curl_calls" || fail "webhook test did not expand header template"
grep -Fq -- '--data port=62000' "$curl_calls" || fail "webhook test did not expand request body"

script_test_output="$(
	printf '{"section":"wan_ct","script":"echo script-ok-$QNATTER_SCRIPT_INSTANCE-$QNATTER_SCRIPT_PROTOCOL-$QNATTER_SCRIPT_OUTER_PORT"}\n' | \
		QNATTER_JSONFILTER_BIN="$jsonfilter_bin" \
		QNATTER_TIMEOUT_BIN="$tmp/missing-timeout" \
		"$rpcd" call completion_script_test
)"
[ "$script_test_output" = '{"ok":true,"output":"script-ok-wan_ct-tcp-62000"}' ] || fail "script test output = $script_test_output"

: > "$run_dir/wan_ct.json"
: > "$run_dir/wan_ct.env"
: > "$run_dir/wan_ct.notify"
: > "$log_dir/wan_ct.log"

rename_output="$(
	printf '{"old":"wan_ct","new":"wan_new"}\n' | \
		QNATTER_UCI_BIN="$uci_bin" \
		QNATTER_MV_BIN="$mv_bin" \
		QNATTER_INIT_BIN="$init_bin" \
		QNATTER_RUN_DIR="$run_dir" \
		QNATTER_LOG_DIR="$log_dir" \
		"$rpcd" call rename_instance
)"
[ "$rename_output" = '{"ok":true,"name":"wan_new"}' ] || fail "rename output = $rename_output"
grep -Fqx 'rename qnatter.wan_ct=wan_new' "$uci_calls" || fail "rename did not rename qnatter UCI section"
grep -Fqx 'delete firewall.qnatter_wan_ct' "$uci_calls" || fail "rename did not delete old auto firewall section"
grep -Fqx 'commit qnatter' "$uci_calls" || fail "rename did not commit qnatter config"
grep -Fqx 'commit firewall' "$uci_calls" || fail "rename did not commit firewall config"
grep -Fqx "$run_dir/wan_ct.json $run_dir/wan_new.json" "$mv_calls" || fail "rename did not move status file"
grep -Fqx "$run_dir/wan_ct.env $run_dir/wan_new.env" "$mv_calls" || fail "rename did not move env file"
grep -Fqx "$run_dir/wan_ct.notify $run_dir/wan_new.notify" "$mv_calls" || fail "rename did not move notify wrapper"
grep -Fqx "$log_dir/wan_ct.log $log_dir/wan_new.log" "$mv_calls" || fail "rename did not move log file"
grep -Fqx 'reload' "$init_calls" || fail "rename did not reload QNatter"

toggle_output="$(
	printf '{"instance":"wan_ct"}\n' | \
		QNATTER_UCI_BIN="$uci_bin" \
		QNATTER_INIT_BIN="$init_bin" \
		"$rpcd" call toggle_instance
)"
[ "$toggle_output" = '{"ok":true,"enabled":1}' ] || fail "toggle_instance output = $toggle_output"
grep -Fqx 'set qnatter.wan_ct.enabled=1' "$uci_calls" || fail "toggle_instance did not enable the instance"
grep -Fqx 'reload' "$init_calls" || fail "toggle_instance did not reload QNatter"

invalid_rename_output="$(
	printf '{"old":"wan_ct","new":"bad-name"}\n' | QNATTER_UCI_BIN="$uci_bin" "$rpcd" call rename_instance
)"
printf '%s' "$invalid_rename_output" | grep -Fq '"ok":false' || fail "invalid rename did not fail"

printf 'qnatter rpcd checks passed\n'
