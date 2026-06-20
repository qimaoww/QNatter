#!/bin/sh

set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	exit 1
}

assert_file() {
	[ -f "$ROOT/$1" ] || fail "missing file: $1"
}

assert_contains() {
	file="$1"
	pattern="$2"
	grep -Eq "$pattern" "$ROOT/$file" || fail "$file does not contain pattern: $pattern"
}

assert_not_contains() {
	file="$1"
	pattern="$2"
	if grep -Eq "$pattern" "$ROOT/$file"; then
		fail "$file contains forbidden pattern: $pattern"
	fi
}

assert_po_translation() {
	msgid="$1"
	msgstr="$2"
	grep -Fqx "msgid \"$msgid\"" "$ROOT/luci-app-natter/po/zh_Hans/natter.po" || \
		fail "missing zh_Hans msgid: $msgid"
	grep -Fqx "msgstr \"$msgstr\"" "$ROOT/luci-app-natter/po/zh_Hans/natter.po" || \
		fail "missing zh_Hans msgstr for $msgid: $msgstr"
}

assert_no_path() {
	[ ! -e "$ROOT/$1" ] || fail "unexpected path exists: $1"
}

assert_file natter/Makefile
assert_file natter/files/natter.init
assert_file natter/files/natter-common.sh
assert_file natter/files/natter-qbittorrent.sh
assert_file natter/files/natter-notify
assert_file natter/files/natter-run
assert_file natter/files/natter.config
assert_file natter/files/natter.uci-default

[ -x "$ROOT/natter/files/natter.uci-default" ] || fail "natter uci-defaults source is not executable"

assert_file luci-app-natter/Makefile
assert_file luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js
assert_file luci-app-natter/htdocs/luci-static/resources/view/natter/status.js
assert_file luci-app-natter/htdocs/luci-static/resources/view/natter/log.js
assert_file luci-app-natter/root/usr/share/luci/menu.d/luci-app-natter.json
assert_file luci-app-natter/root/usr/share/rpcd/acl.d/luci-app-natter.json
assert_file luci-app-natter/root/usr/libexec/rpcd/luci.natter
assert_file luci-app-natter/root/usr/libexec/natter-status
assert_file luci-app-natter/root/usr/libexec/natter-log
assert_file go-natter/go.mod
assert_file go-natter/cmd/natter/main.go
assert_file go-natter/internal/app/app.go
assert_file go-natter/internal/config/config.go
assert_file go-natter/internal/stun/message.go

assert_no_path luci-app-natter/luci-app-natter
assert_no_path luci-app-natter/luasrc
assert_no_path luci-app-natter/root/www/luci-static/natter

[ -x "$ROOT/luci-app-natter/root/usr/libexec/rpcd/luci.natter" ] || fail "rpcd helper is not executable"
[ -x "$ROOT/luci-app-natter/root/usr/libexec/natter-status" ] || fail "status helper is not executable"
[ -x "$ROOT/luci-app-natter/root/usr/libexec/natter-log" ] || fail "log helper is not executable"

assert_not_contains luci-app-natter/Makefile 'luci-compat'
assert_not_contains luci-app-natter/Makefile 'LUCI_DEPENDS:=.*\+luci-compat'
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "^'require form';"
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "^'require fs';"
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "^'require tools\\.widgets as widgets';"
assert_not_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js 'cbi\('
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js 'natter-theme-aurora'
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "L\\.resource\\('natter/natter\\.css'\\)"
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js 'hideInGrid'
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "hideInGrid\\(s\\.option\\(widgets\\.DeviceSelect, 'bind_value'"
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "widgets\\.DeviceSelect, 'bind_value', _\\('WAN interface'\\)"
assert_not_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "form\\.ListValue, 'runtime'"
assert_not_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "/usr/bin/python3"
assert_not_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js 'tools\.python'
assert_not_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "o\\.value\\('python'"
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js 'o\.rmempty = true'
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js 'o\.nocreate = true'
assert_not_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "s\\.option\\(form\\.ListValue, 'network'"
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "fs\\.stat\\('/usr/bin/socat'\\)"
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "fs\\.stat\\('/usr/bin/gost'\\)"
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js 'tools\.socat'
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js 'tools\.gost'
assert_not_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "o\\.value\\('iptables'"
assert_not_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "o\\.value\\('iptables-snat'"
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "hideInGrid\\(s\\.option\\(form\\.Value, 'target_ip'"
assert_not_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js 'Port 0 forwards to the Natter mapped internal port\.'
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "hideInGrid\\(s\\.option\\(form\\.DynamicList, 'stun_server'"
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "hideInGrid\\(s\\.option\\(form\\.Value, 'notify_script'"
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/instances.js "hideInGrid\\(s\\.option\\(form\\.Flag, 'qbittorrent_enabled'"
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/status.js 'natter-theme-aurora'
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/status.js "expect: \\{ '': \\{ instances: \\[\\] \\} \\}"
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/status.js 'data\.instances'
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/log.js 'natter-theme-aurora'
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/log.js "expect: \\{ '': \\{ instances: \\[\\] \\} \\}"
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/log.js "expect: \\{ '': \\{ log: '' \\} \\}"
assert_contains luci-app-natter/htdocs/luci-static/resources/view/natter/log.js 'data\.log'
assert_contains luci-app-natter/root/usr/share/luci/menu.d/luci-app-natter.json '"type"[[:space:]]*:[[:space:]]*"view"'
assert_contains luci-app-natter/root/usr/libexec/rpcd/luci.natter '"instance":"String"'
assert_contains luci-app-natter/root/usr/libexec/rpcd/luci.natter '"lines":"Integer"'
assert_contains luci-app-natter/root/usr/libexec/natter-status 'grep -Fx'
assert_contains luci-app-natter/htdocs/luci-static/resources/natter/natter.css 'theme-argon'
assert_contains luci-app-natter/htdocs/luci-static/resources/natter/natter.css 'natter-theme-aurora'
assert_contains luci-app-natter/htdocs/luci-static/resources/natter/natter.css 'overflow-wrap: anywhere'
assert_contains luci-app-natter/htdocs/luci-static/resources/natter/natter.css 'min-width: 0'
assert_contains luci-app-natter/htdocs/luci-static/resources/natter/natter.css 'max-width: 100%'
assert_contains luci-app-natter/htdocs/luci-static/resources/natter/natter.css 'word-break: keep-all'
assert_contains luci-app-natter/htdocs/luci-static/resources/natter/natter.css 'overflow-x: auto'

assert_contains natter/files/natter.init 'config_get network "\$section" network "wan"'
assert_not_contains natter/files/natter.init 'network_get_device device "\$network"'
assert_not_contains natter/files/natter.init 'network_get_ipaddr ipaddr "\$network"'
assert_not_contains natter/files/natter.init 'printf .*0\.0\.0\.0'
assert_not_contains natter/files/natter.init 'ip rule add'
assert_contains natter/files/natter.init 'bind=\$\{resolved_bind:-default route\}'
assert_contains natter/files/natter.init 'config_get bind_value "\$section" bind_value ""'
assert_contains natter/files/natter.init '\[ -n "\$bind_value" \] && return 0'
assert_contains natter/files/natter.hotplug 'config_get bind_value "\$section" bind_value'
assert_contains natter/files/natter.hotplug 'if \[ -n "\$bind_value" \]; then'
assert_contains natter/files/natter.hotplug '\[ "\$bind_value" = "\$DEVICE" \] && MATCHED=1'
assert_contains natter/files/natter.config "option forward_method 'auto'"
assert_not_contains natter/files/natter.config "^[[:space:]]*list[[:space:]]+stun_server"
assert_contains natter/files/natter-common.sh 'natter_forward_method_or_auto'
assert_contains natter/files/natter-common.sh '\[ "\$forward_method" != "auto" \]'
assert_contains natter/files/natter.init 'NATTER_STATUS_FILE'
assert_not_contains natter/files/natter.config 'option runtime'
assert_not_contains natter/files/natter.init 'config_get runtime'
assert_not_contains natter/files/natter.init 'NATTER_RUNTIME'
assert_not_contains natter/files/natter.init 'PROG="/usr/bin/natter.py"'
assert_contains natter/files/natter.init 'qbittorrent_target_ip'
assert_not_contains natter/files/natter-run 'NATTER_PY_BIN'
assert_not_contains natter/files/natter-run 'NATTER_RUNTIME'
assert_contains natter/files/natter-run 'NATTER_GO_BIN:-/usr/bin/Natter'
assert_no_path natter/files/Natter
assert_no_path natter/files/natter-python-wrapper.py
assert_contains natter/files/natter-qbittorrent.sh 'natter_qb_select_listen_port'
assert_contains natter/files/natter-notify 'api/v2/auth/login'
assert_contains natter/files/natter-notify 'api/v2/app/setPreferences'
assert_not_contains natter/Makefile 'DEPENDS:=.*\+python3'
assert_contains natter/Makefile 'DEPENDS:=.*\+curl'
assert_contains natter/Makefile 'DEPENDS:=.*\+nftables'
assert_contains natter/Makefile 'DEPENDS:=.*\+firewall4'
assert_contains natter/Makefile 'DEPENDS:=.*\+kmod-nft-nat'
assert_contains natter/Makefile 'go build .* -o \$\(PKG_BUILD_DIR\)/natter-go ./cmd/natter'
assert_contains natter/Makefile '\$\(INSTALL_BIN\) \$\(PKG_BUILD_DIR\)/natter-go \$\(1\)/usr/bin/natter-go'
assert_contains natter/Makefile '\$\(LN\) natter-go \$\(1\)/usr/bin/Natter'
natter_release="$(sed -n 's/^PKG_RELEASE:=//p' "$ROOT/natter/Makefile")"
[ "$natter_release" -gt 1 ] || fail "natter Go package release must be greater than 1"
assert_contains natter/Makefile '\$\(INSTALL_CONF\) ./files/natter.config \$\(1\)/etc/config/natter.default'
assert_contains natter/Makefile '\$\(INSTALL_DIR\) \$\(1\)/etc/uci-defaults'
assert_contains natter/Makefile '\$\(INSTALL_BIN\) ./files/natter.uci-default \$\(1\)/etc/uci-defaults/99-natter'
assert_not_contains natter/Makefile '\$\(INSTALL_CONF\) ./files/natter.config \$\(1\)/etc/config/natter$'
assert_not_contains natter/Makefile '\+iptables-nft'
assert_not_contains natter/Makefile '\+socat'
assert_not_contains natter/Makefile '\+gost'
assert_not_contains natter/Makefile '\+python3-light'
assert_not_contains natter/Makefile 'natter.py'
assert_not_contains natter/Makefile './files/Natter'
assert_not_contains natter/Makefile 'natter-python-wrapper.py'
assert_contains luci-app-natter/Makefile 'LUCI_DEPENDS:=.*\+natter'
assert_contains luci-app-natter/Makefile 'LUCI_DEPENDS:=.*\+luci-base'
assert_contains luci-app-natter/Makefile 'LUCI_DEPENDS:=.*\+rpcd'
assert_contains luci-app-natter/Makefile 'LUCI_DEPENDS:=.*\+uhttpd'
assert_contains luci-app-natter/Makefile 'LUCI_DEPENDS:=.*\+uhttpd-mod-ubus'
assert_contains luci-app-natter/Makefile 'LUCI_DEPENDS:=.*\+uci'
assert_contains natter/files/natter.uci-default 'NATTER_UCI_CONFIG:=/etc/config/natter'
assert_contains natter/files/natter.uci-default 'NATTER_UCI_DEFAULT:=/etc/config/natter.default'
assert_contains natter/files/natter.uci-default '\[ -e "\$NATTER_UCI_CONFIG" \] && exit 0'
assert_contains natter/files/natter.uci-default 'cp "\$NATTER_UCI_DEFAULT" "\$NATTER_UCI_CONFIG"'

assert_po_translation 'Global Settings' '全局设置'
assert_po_translation 'Expose ports behind full-cone NAT with optional forwarding and qBittorrent port updates.' '通过全锥形 NAT 暴露端口，并可选转发和更新 qBittorrent 监听端口。'
assert_po_translation 'Runtime' '运行时'
assert_po_translation 'WAN interface' 'WAN 接口'
assert_po_translation 'Leave empty to bind to the default WAN device.' '留空则自动绑定默认 WAN 设备。'
assert_po_translation 'Forward method' '转发方式'
assert_po_translation 'Forward target port' '转发目标端口'
assert_not_contains luci-app-natter/po/zh_Hans/natter.po 'Port 0 forwards to the Natter mapped internal port\.'
assert_po_translation 'Port 0 forwards to the port Natter reports after punching.' '端口 0 会转发到 Natter 打洞后报告的端口。'
assert_po_translation 'Natter Status' 'Natter 状态'
assert_po_translation 'Public address' '公网地址'
assert_po_translation 'Internal address' '内部地址'
assert_po_translation 'Waiting for mapping' '等待映射'
assert_po_translation 'Natter Logs' 'Natter 日志'
assert_po_translation 'Clear logs' '清空日志'
assert_po_translation 'Logs cleared' '日志已清空'

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
cp "$ROOT/natter/files/natter-qbittorrent.sh" "$tmp/"

(
	. "$tmp/natter-qbittorrent.sh"
	[ "$(natter_qb_select_listen_port 5000 62000 0)" = "62000" ] || exit 11
	[ "$(natter_qb_select_listen_port 5000 62000 51413)" = "51413" ] || exit 12
	qb_port_err="$tmp/qb-port.err"
	[ "$(natter_qb_select_listen_port 5000 62000 999999999999999999999999 2>"$qb_port_err")" = "62000" ] || exit 19
	[ ! -s "$qb_port_err" ] || exit 20
	[ "$(natter_qb_preferences_json 62000)" = '{"listen_port":62000}' ] || exit 13
	[ "$(natter_qb_normalize_url 'http://127.0.0.1:8080///')" = "http://127.0.0.1:8080" ] || exit 18

	env_file="$tmp/qb-env"
	natter_qb_write_notify_env "$env_file" \
		QBITTORRENT_URL "http://127.0.0.1:8080/path with spaces" \
		QBITTORRENT_USERNAME "admin user" \
		QBITTORRENT_PASSWORD "pa'ss word"
	# shellcheck disable=SC1090
	. "$env_file"
	[ "$QBITTORRENT_URL" = "http://127.0.0.1:8080/path with spaces" ] || exit 14
	[ "$QBITTORRENT_USERNAME" = "admin user" ] || exit 15
	[ "$QBITTORRENT_PASSWORD" = "pa'ss word" ] || exit 16

	chmod 0644 "$env_file"
	natter_qb_write_notify_env "$env_file" QBITTORRENT_PASSWORD "secret"
	mode="$(ls -l "$env_file" | awk '{ print $1 }')"
	case "$mode" in
		-rw-------) : ;;
		*) exit 17 ;;
	esac
)

(
	. "$ROOT/natter/files/natter-common.sh"
	old_path="$PATH"

	[ "$(natter_slug wan_ct)" = "wan_ct" ] || exit 26
	[ "$(natter_slug 'wan-ct')" = "wan_ct" ] || exit 27
	json_input="$(printf 'line "one"\\\nline\rtwo\tend')"
	[ "$(natter_json_escape "$json_input")" = 'line \"one\"\\\nline\rtwo\tend' ] || exit 28

	NATTER_FORWARD_METHOD=iptables
	natter_build_args
	case "$NATTER_ARGS" in *iptables*|*'-m'*) exit 21 ;; esac

	PATH=/nonexistent NATTER_FORWARD_METHOD=socat
	natter_build_args
	case "$NATTER_ARGS" in *socat*|*'-m'*) exit 22 ;; esac

	PATH=/nonexistent NATTER_FORWARD_METHOD=gost
	natter_build_args
	case "$NATTER_ARGS" in *gost*|*'-m'*) exit 24 ;; esac
	PATH="$old_path"

	mkdir -p "$tmp/bin"
	printf '#!/bin/sh\n' > "$tmp/bin/socat"
	chmod 0755 "$tmp/bin/socat"
	printf '#!/bin/sh\n' > "$tmp/bin/gost"
	chmod 0755 "$tmp/bin/gost"

	PATH="$tmp/bin" NATTER_FORWARD_METHOD=socat
	natter_build_args
	case "$NATTER_ARGS" in *socat*) ;; *) exit 23 ;; esac

	PATH="$tmp/bin" NATTER_FORWARD_METHOD=gost
	natter_build_args
	case "$NATTER_ARGS" in *gost*) ;; *) exit 25 ;; esac
)

sh -n "$ROOT/natter/files/natter-common.sh"
sh -n "$ROOT/natter/files/natter-qbittorrent.sh"
sh -n "$ROOT/natter/files/natter-notify"
sh -n "$ROOT/natter/files/natter-run"
sh -n "$ROOT/natter/files/natter.init"
sh -n "$ROOT/tests/test_natter_hotplug.sh"
sh -n "$ROOT/luci-app-natter/root/usr/libexec/natter-status"
sh -n "$ROOT/luci-app-natter/root/usr/libexec/natter-log"
sh -n "$ROOT/luci-app-natter/root/usr/libexec/rpcd/luci.natter"
sh -n "$ROOT/tests/test_natter_init.sh"
sh -n "$ROOT/tests/test_natter_log.sh"
sh -n "$ROOT/tests/test_natter_notify.sh"
sh -n "$ROOT/tests/test_natter_rpcd.sh"
sh -n "$ROOT/tests/test_natter_status.sh"
sh -n "$ROOT/tests/test_natter_uci_default.sh"

(
	printf '#!/bin/sh\necho "go:$*"\n' > "$tmp/natter-go-bin"
	chmod 0755 "$tmp/natter-go-bin"

	NATTER_GO_BIN="$tmp/natter-go-bin" \
		"$ROOT/natter/files/natter-run" "$tmp/default-runtime.log" alpha beta
	grep -qx 'go:alpha beta' "$tmp/default-runtime.log" || fail "natter-run default runtime must execute natter-go"

	NATTER_RUNTIME=python NATTER_GO_BIN="$tmp/natter-go-bin" \
		"$ROOT/natter/files/natter-run" "$tmp/python-runtime.log" gamma
	grep -qx 'go:gamma' "$tmp/python-runtime.log" || fail "natter-run legacy python runtime must execute natter-go"
)

(cd "$ROOT/go-natter" && go test ./...)
"$ROOT/tests/test_natter_hotplug.sh"
"$ROOT/tests/test_natter_init.sh"
"$ROOT/tests/test_natter_log.sh"
"$ROOT/tests/test_natter_notify.sh"
"$ROOT/tests/test_natter_rpcd.sh"
"$ROOT/tests/test_natter_status.sh"
"$ROOT/tests/test_natter_uci_default.sh"

dummy_natter_go="$tmp/natter-go"
archive="$tmp/natter-openwrt-direct.tar.gz"
printf '#!/bin/sh\n' > "$dummy_natter_go"
chmod 0755 "$dummy_natter_go"
if tar -tzf "$archive" | grep -qx 'usr/bin/natter.py'; then
fi
if tar -tzf "$archive" | grep -qx 'usr/share/natter/natter-python-wrapper.py'; then
fi
tar -tvzf "$archive" | awk '$6 == "usr/bin/natter-go" { print $1 }' | grep -q '^-rwxr-xr-x$' || \
tar -tvzf "$archive" | awk '$6 == "usr/bin/Natter" { print $1 }' | grep -q '^lrwxrwxrwx$' || \
tar -tvzf "$archive" | awk '$6 == "etc/uci-defaults/99-natter" { print $1 }' | grep -q '^-rwxr-xr-x$' || \
if tar -tzf "$archive" | grep -Eq '^\./?$'; then
fi
tar -tvzf "$archive" | awk '$6 ~ /^(etc|usr|www)\/?$/ { print $1 }' | grep -q '^drwxr-xr-x' || \

printf 'natter package static checks passed\n'
