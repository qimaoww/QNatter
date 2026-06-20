#!/bin/sh

set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	exit 1
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

status_bin="$tmp/natter-status"
log_bin="$tmp/natter-log"
log_calls="$tmp/log-calls.txt"

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
		;;
	clear)
		printf '{"ok":true}\n'
		;;
esac
EOF
chmod 0755 "$log_bin"

rpcd="$ROOT/luci-app-natter/root/usr/libexec/rpcd/luci.natter"

status_output="$(
	printf '{}\n' | NATTER_STATUS_BIN="$status_bin" NATTER_LOG_BIN="$log_bin" "$rpcd" call status
)"
[ "$status_output" = '{"instances":[{"name":"wan_ct","running":true}]}' ] || fail "status output = $status_output"

log_output="$(
	printf '{"instance":"wan_ct","lines":25}\n' | NATTER_STATUS_BIN="$status_bin" NATTER_LOG_BIN="$log_bin" "$rpcd" call log
)"
expected_log='{"log":"alpha \"quoted\"\nbeta \\ slash"}'
[ "$log_output" = "$expected_log" ] || fail "log output was not escaped correctly: $log_output"
grep -Fqx 'read wan_ct 25' "$log_calls" || fail "log helper was not called with requested instance and lines"

clear_output="$(
	printf '{"instance":"wan_ct"}\n' | NATTER_STATUS_BIN="$status_bin" NATTER_LOG_BIN="$log_bin" "$rpcd" call clear_log
)"
[ "$clear_output" = '{"ok":true}' ] || fail "clear output = $clear_output"
grep -Fqx 'clear wan_ct 200' "$log_calls" || fail "clear helper was not called with requested instance"

printf 'natter rpcd checks passed\n'
