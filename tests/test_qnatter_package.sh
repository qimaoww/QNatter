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

assert_css_block_contains() {
	file="$1"
	selector="$2"
	text="$3"
	awk -v selector="$selector" -v text="$text" '
		index($0, selector) {
			found = 1
			in_block = 1
			next
		}
		in_block && index($0, "}") {
			in_block = 0
		}
		in_block && index($0, text) {
			seen = 1
		}
		END {
			exit(found && seen ? 0 : 1)
		}
	' "$ROOT/$file" || fail "$file selector $selector does not contain text: $text"
}

assert_option_block_contains() {
	file="$1"
	option="$2"
	text="$3"
	awk -v option="'$option'" -v text="$text" '
		index($0, "s.option") && index($0, option) {
			found = 1
			in_block = 1
			next
		}
		in_block && /^[[:space:]]*o = / {
			in_block = 0
		}
		in_block && index($0, text) {
			seen = 1
		}
		END {
			exit(found && seen ? 0 : 1)
		}
	' "$ROOT/$file" || fail "$file option $option does not contain text: $text"
}

assert_option_block_not_contains() {
	file="$1"
	option="$2"
	text="$3"
	awk -v option="'$option'" -v text="$text" '
		index($0, "s.option") && index($0, option) {
			found = 1
			in_block = 1
			next
		}
		in_block && /^[[:space:]]*o = / {
			in_block = 0
		}
		in_block && index($0, text) {
			seen = 1
		}
		END {
			exit(found && !seen ? 0 : 1)
		}
	' "$ROOT/$file" || fail "$file option $option contains forbidden text: $text"
}

assert_po_translation() {
	msgid="$1"
	msgstr="$2"
	grep -Fqx "msgid \"$msgid\"" "$ROOT/luci-app-qnatter/po/zh_Hans/qnatter.po" || \
		fail "missing zh_Hans msgid: $msgid"
	grep -Fqx "msgstr \"$msgstr\"" "$ROOT/luci-app-qnatter/po/zh_Hans/qnatter.po" || \
		fail "missing zh_Hans msgstr for $msgid: $msgstr"
}

assert_no_path() {
	[ ! -e "$ROOT/$1" ] || fail "unexpected path exists: $1"
}

assert_file qnatter/Makefile
assert_file qnatter/files/qnatter.init
assert_file qnatter/files/qnatter-common.sh
assert_file qnatter/files/qnatter-qbittorrent.sh
assert_file qnatter/files/qnatter-notify
assert_file qnatter/files/qnatter-run
assert_file qnatter/files/qnatter.config
assert_file qnatter/files/qnatter.uci-default

[ -x "$ROOT/qnatter/files/qnatter.uci-default" ] || fail "qnatter uci-defaults source is not executable"

assert_file luci-app-qnatter/Makefile
assert_file luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js
assert_file luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js
assert_file luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js
assert_file luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js
assert_file luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/log.js
assert_file luci-app-qnatter/root/usr/share/luci/menu.d/luci-app-qnatter.json
assert_file luci-app-qnatter/root/usr/share/rpcd/acl.d/luci-app-qnatter.json
assert_file luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter
assert_file luci-app-qnatter/root/usr/libexec/qnatter-status
assert_file luci-app-qnatter/root/usr/libexec/qnatter-log
assert_file go-qnatter/go.mod
assert_file go-qnatter/cmd/qnatter/main.go
assert_file go-qnatter/internal/app/app.go
assert_file go-qnatter/internal/config/config.go
assert_file go-qnatter/internal/stun/message.go

assert_no_path luci-app-qnatter/luci-app-qnatter
assert_no_path luci-app-qnatter/luasrc
assert_no_path luci-app-qnatter/root/www/luci-static/qnatter

[ -x "$ROOT/luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter" ] || fail "rpcd helper is not executable"
[ -x "$ROOT/luci-app-qnatter/root/usr/libexec/qnatter-status" ] || fail "status helper is not executable"
[ -x "$ROOT/luci-app-qnatter/root/usr/libexec/qnatter-log" ] || fail "log helper is not executable"

assert_not_contains luci-app-qnatter/Makefile 'luci-compat'
assert_not_contains luci-app-qnatter/Makefile 'LUCI_DEPENDS:=.*\+luci-compat'
assert_contains luci-app-qnatter/Makefile '^PKG_VERSION:=1\.1\.0$'
assert_contains luci-app-qnatter/Makefile '^PKG_RELEASE:=1$'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "^'require form';"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "^'require fs';"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "^'require rpc';"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "^'require tools\\.widgets as widgets';"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'cbi\('
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'qnatter-theme-aurora'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "L\\.resource\\('qnatter/qnatter\\.css'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "\\?v=1\\.1\\.0-r1"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "\\?v=1\\.1\\.0-r1"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js "\\?v=1\\.1\\.0-r1"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'hideInGrid'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'option\.keylist'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'indexOf\(String\(value\)\)'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "return E\\('button'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "hideInGrid\\(s\\.option\\(widgets\\.DeviceSelect, 'bind_value'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "widgets\\.DeviceSelect, 'bind_value', _\\('WAN interface'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "form\\.Flag, 'hot_reload', _\\('Hot reload'\\)"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "form\\.Value, 'label'"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "form\\.ListValue, 'runtime'"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "/usr/bin/python3"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'tools\.python'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "o\\.value\\('python'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'o\.rmempty = true'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'o\.nocreate = true'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "s\\.option\\(form\\.ListValue, 'network'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "fs\\.stat\\('/usr/bin/socat'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "fs\\.stat\\('/usr/bin/gost'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'tools\.socat'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'tools\.gost'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "o\\.value\\('iptables'"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "o\\.value\\('iptables-snat'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "o\\.depends\\('qbittorrent_forward', '0'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "o\\.depends\\('qbittorrent_enabled', '0'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "hideInGrid\\(s\\.option\\(form\\.Flag, 'auto_firewall'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "hideInGrid\\(s\\.option\\(form\\.Value, 'target_ip'"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js target_ip "o.depends('qbittorrent_enabled', '0')"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js target_ip "o.depends('qbittorrent_forward', '0')"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js target_port "o.depends('qbittorrent_enabled', '0')"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js target_port "o.depends('qbittorrent_forward', '0')"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'Port 0 forwards to the QNatter mapped internal port\.'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "form\\.DynamicList, 'stun_server'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "form\\.DynamicList, 'stun_server'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "renderStunDynamicList"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "filterListValues"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "refreshDynamicListChoices"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "o\\.placeholder = _\\('Custom'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "Try these instance STUN servers before the global list"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "custom_placeholder = 'host\\[:port\\]'"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "Select a candidate or enter a custom host\\[:port\\]"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "uniqueListValues"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js "form\\.DynamicList"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js "form\\.TextValue"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js "serverDynamicList\\(s, 'tcp_server'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js "serverDynamicList\\(s, 'udp_server'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js "o\\.placeholder = _\\('Custom'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js "DEFAULT_TCP_STUN_SERVERS"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js "DEFAULT_UDP_STUN_SERVERS"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js "renderServerDynamicList"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js "refreshDynamicListChoices"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js "filterValues"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js "TCP STUN servers"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js "UDP STUN servers"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js "Duplicate STUN server"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js "uniqueServerValues"
assert_contains luci-app-qnatter/root/usr/share/luci/menu.d/luci-app-qnatter.json '"path": "qnatter/stun"'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "hideInGrid\\(s\\.option\\(form\\.Value, 'notify_script'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "hideInGrid\\(s\\.option\\(form\\.Flag, 'cloudflare_enabled'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "hideInGrid\\(s\\.option\\(form\\.Value, 'cloudflare_api_token'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "hideInGrid\\(s\\.option\\(form\\.ListValue, 'cloudflare_zone_id'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "hideInGrid\\(s\\.option\\(form\\.ListValue, 'cloudflare_record_id'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'callCloudflareZones'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'callCloudflareSrvRecords'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'callRenameInstance'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "rename_instance"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "form\\.Value, '_rename_to'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "form\\.DummyValue, '_rename_instance'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "_\\(result && result\\.error \\? result\\.error : 'Rename failed'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "hideInGrid\\(s\\.option\\(form\\.Flag, 'qbittorrent_enabled'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'qnatter-theme-aurora'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js "expect: \\{ '': \\{ instances: \\[\\] \\} \\}"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'data\.instances'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js "method: 'toggle_instance'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js "callToggleInstance\\(name\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js "toggleInstance\\(name, this\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js "_\\('RUNNING'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'cardByName'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'fieldByName'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'refreshInFlight'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'document\.hidden'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'setTimeout'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'updateCard'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js "\\?v=1\\.1\\.0-r1"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'grid\.replaceChildren\.apply'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js "_\\('Group'\\)"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'item\.label'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'item\.name[[:space:]]*\|\|[[:space:]]*item\.instance'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/log.js 'qnatter-theme-aurora'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/log.js "expect: \\{ '': \\{ instances: \\[\\] \\} \\}"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/log.js "expect: \\{ '': \\{ log: '' \\} \\}"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/log.js 'data\.log'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/log.js "\\?v=1\\.1\\.0-r1"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/log.js 'label'
assert_contains luci-app-qnatter/root/usr/share/luci/menu.d/luci-app-qnatter.json '"type"[[:space:]]*:[[:space:]]*"view"'
assert_contains luci-app-qnatter/root/usr/share/luci/menu.d/luci-app-qnatter.json '"path"[[:space:]]*:[[:space:]]*"qnatter/instances-v11"'
assert_not_contains luci-app-qnatter/root/usr/libexec/qnatter-status 'json_pair label'
assert_not_contains luci-app-qnatter/root/usr/libexec/qnatter-status 'config_get label'
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter '"instance":"String"'
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter '"lines":"Integer"'
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter '"cloudflare_zones":\{"section":"String","token":"String"\}'
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter '"cloudflare_srv_records":\{"section":"String","zone_id":"String","token":"String"\}'
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter '"rename_instance":\{"old":"String","new":"String"\}'
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter '"toggle_instance":\{"instance":"String"\}'
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter 'valid_section_name "\$instance"'
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter '\$QNATTER_INIT_BIN" reload'
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter 'cloudflare_api_get'
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter 'dns_records\?type=SRV'
assert_contains luci-app-qnatter/root/usr/share/rpcd/acl.d/luci-app-qnatter.json '"cloudflare_zones"'
assert_contains luci-app-qnatter/root/usr/share/rpcd/acl.d/luci-app-qnatter.json '"cloudflare_srv_records"'
assert_contains luci-app-qnatter/root/usr/share/rpcd/acl.d/luci-app-qnatter.json '"rename_instance"'
assert_contains luci-app-qnatter/root/usr/share/rpcd/acl.d/luci-app-qnatter.json '"toggle_instance"'
assert_contains luci-app-qnatter/root/usr/libexec/qnatter-status 'grep -Fx'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'theme-argon'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'qnatter-theme-aurora'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css ':root:not\(\[data-darkmode="true"\]\) \.qnatter-theme-aurora'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '\[data-darkmode="true"\] \.qnatter-theme-aurora'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'var\(--surface-overlay'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'var\(--brand'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'var\(--text'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'var\(--hairline'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'var\(--success-surface'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '\.qnatter-theme-aurora \.qnatter-card-head'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '\.qnatter-theme-aurora \.qnatter-card h3'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '\.qnatter-theme-aurora \.qnatter-card dl'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '\.qnatter-theme-aurora \.qnatter-card dd'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '\.qnatter-theme-aurora \.qnatter-card h3 \.cbi-button'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '\.qnatter-theme-aurora\.qnatter-form-page \.cbi-section'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '\.qnatter-theme-aurora\.qnatter-form-page \.cbi-section-table'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '\.qnatter-theme-aurora\.qnatter-log-page \.qnatter-log'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'text-transform: none'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'border-radius: 14px'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'var\(--hover-faint'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'font-variant-numeric: tabular-nums'
assert_css_block_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '.qnatter-theme-aurora .qnatter-card h3 .cbi-button' 'background: var(--surface-sunken, rgba(248, 250, 253, 0.96));'
assert_css_block_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '.qnatter-theme-aurora .qnatter-card h3 .cbi-button' 'border: 1px solid var(--hairline, rgba(25, 35, 50, 0.12));'
assert_css_block_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '.qnatter-theme-aurora .qnatter-card h3 .cbi-button' 'box-shadow: none;'
assert_css_block_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '.qnatter-theme-aurora .qnatter-card h3 .cbi-button' 'color: var(--text-muted, #667085);'
assert_css_block_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '.qnatter-theme-aurora .qnatter-card h3 .cbi-button' 'align-items: center;'
assert_css_block_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '.qnatter-theme-aurora .qnatter-card h3 .cbi-button' 'display: inline-flex;'
assert_css_block_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '.qnatter-theme-aurora .qnatter-card h3 .cbi-button' 'font-weight: 600;'
assert_css_block_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '.qnatter-theme-aurora .qnatter-card h3 .cbi-button' 'min-height: 22px;'
assert_css_block_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '.qnatter-theme-aurora .qnatter-card h3 .cbi-button' 'vertical-align: middle;'
assert_css_block_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '.qnatter-theme-aurora .qnatter-card h3 .cbi-button::before' 'content: "↻";'
assert_css_block_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '.qnatter-theme-aurora .qnatter-card h3 .cbi-button::before' 'line-height: 1;'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'text-transform: uppercase'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'rgba\(18, 24, 38'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'color: #edf6ff'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'overflow-wrap: anywhere'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'min-width: 0'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'max-width: 100%'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'word-break: keep-all'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'overflow-x: auto'

assert_contains qnatter/files/qnatter.init 'config_get network "\$section" network "wan"'
assert_not_contains qnatter/files/qnatter.init 'network_get_device device "\$network"'
assert_not_contains qnatter/files/qnatter.init 'network_get_ipaddr ipaddr "\$network"'
assert_not_contains qnatter/files/qnatter.init 'printf .*0\.0\.0\.0'
assert_not_contains qnatter/files/qnatter.init 'ip rule add'
assert_contains qnatter/files/qnatter.init 'bind=\$\{resolved_bind:-default route\}'
assert_contains qnatter/files/qnatter.init 'config_get bind_value "\$section" bind_value ""'
assert_contains qnatter/files/qnatter.init '\[ -n "\$bind_value" \] && return 0'
assert_contains qnatter/files/qnatter.hotplug 'config_get bind_value "\$section" bind_value'
assert_contains qnatter/files/qnatter.hotplug 'if \[ -n "\$bind_value" \]; then'
assert_contains qnatter/files/qnatter.hotplug '\[ "\$bind_value" = "\$DEVICE" \] && MATCHED=1'
assert_contains qnatter/files/qnatter.config "option forward_method 'auto'"
assert_contains qnatter/files/qnatter.config "option hot_reload '1'"
assert_contains qnatter/files/qnatter.config "option route_slot '0'"
assert_not_contains qnatter/files/qnatter.config "option label"
assert_contains qnatter/files/qnatter.config "option auto_firewall '0'"
assert_contains qnatter/files/qnatter.config "option cloudflare_enabled '0'"
assert_contains qnatter/files/qnatter.config "option cloudflare_api_url ''"
assert_contains qnatter/files/qnatter.config "option cloudflare_api_token ''"
assert_contains qnatter/files/qnatter.config "option cloudflare_zone_id ''"
assert_contains qnatter/files/qnatter.config "option cloudflare_record_id ''"
assert_contains qnatter/files/qnatter.config "config stun 'stun'"
assert_contains qnatter/files/qnatter.config "list tcp_server 'fwa.lifesizecloud.com'"
assert_contains qnatter/files/qnatter.config "list tcp_server 'turn.cloud-rtc.com:80'"
assert_contains qnatter/files/qnatter.config "list udp_server 'stun.miwifi.com'"
assert_contains qnatter/files/qnatter.config "list udp_server 'stun.douyucdn.cn:18000'"
assert_not_contains qnatter/files/qnatter.config "^[[:space:]]*list[[:space:]]+stun_server"
assert_contains qnatter/files/qnatter-common.sh 'qnatter_forward_method_or_auto'
assert_contains qnatter/files/qnatter-common.sh '\[ "\$forward_method" != "auto" \]'
assert_contains qnatter/files/qnatter.init 'QNATTER_STATUS_FILE'
assert_contains qnatter/files/qnatter.init 'QNATTER_ROUTE_MARK'
assert_contains qnatter/files/qnatter.init 'qnatter_prepare_route_slots'
assert_contains qnatter/files/qnatter.init 'config_get route_slot "\$section" route_slot ""'
assert_contains qnatter/files/qnatter.init 'QNATTER_LOG_FILE'
assert_contains qnatter/files/qnatter.init 'hot_reload'
assert_contains qnatter/files/qnatter.init 'runtime'
assert_not_contains qnatter/files/qnatter.init 'config_get label'
assert_contains qnatter/files/qnatter.init 'QNATTER_AUTO_FIREWALL'
assert_contains qnatter/files/qnatter.init 'QNATTER_FIREWALL_SECTION'
assert_contains qnatter/files/qnatter.init 'CLOUDFLARE_SRV_ENABLED'
assert_contains qnatter/files/qnatter.init 'CLOUDFLARE_API_URL'
assert_contains qnatter/files/qnatter.init 'CLOUDFLARE_API_TOKEN'
assert_contains qnatter/files/qnatter.init 'CLOUDFLARE_ZONE_ID'
assert_contains qnatter/files/qnatter.init 'CLOUDFLARE_RECORD_ID'
assert_contains qnatter/files/qnatter-notify 'QNATTER_AUTO_FIREWALL'
assert_contains qnatter/files/qnatter-notify 'QNATTER_UCI_BIN'
assert_contains qnatter/files/qnatter-notify 'firewall\.\$\{section\}\.dest_port=\$\{port\}'
assert_contains qnatter/files/qnatter-notify 'update_cloudflare_srv'
assert_contains qnatter/files/qnatter-notify 'Authorization: Bearer'
assert_contains qnatter/files/qnatter-notify 'CLOUDFLARE_ZONE_ID'
assert_contains qnatter/files/qnatter-notify 'CLOUDFLARE_RECORD_ID'
assert_contains qnatter/files/qnatter-notify '\{"type":"SRV","data":\{"port":'
assert_contains qnatter/files/qnatter-notify 'Cloudflare SRV update started'
assert_contains qnatter/files/qnatter-notify 'Cloudflare SRV current record'
assert_contains qnatter/files/qnatter-notify 'Cloudflare SRV port changed'
assert_contains qnatter/files/qnatter-notify 'Cloudflare SRV updated'
assert_contains qnatter/files/qnatter-notify 'QNATTER_LOG_FILE'
assert_not_contains qnatter/files/qnatter.config 'option runtime'
assert_not_contains qnatter/files/qnatter.init 'config_get runtime'
assert_not_contains qnatter/files/qnatter.init 'PROG="/usr/bin/qnatter.py"'
assert_not_contains qnatter/files/qnatter.init 'mapped internal port'
assert_contains qnatter/files/qnatter.init 'qbittorrent_target_ip'
assert_not_contains qnatter/files/qnatter-run 'QNATTER_PY_BIN'
assert_not_contains qnatter/files/qnatter-run 'QNATTER_RUNTIME'
assert_contains qnatter/files/qnatter-run 'QNATTER_GO_BIN:-/usr/bin/QNatter'
assert_no_path qnatter/files/QNatter
assert_no_path qnatter/files/qnatter-python-wrapper.py
assert_contains qnatter/files/qnatter-qbittorrent.sh 'qnatter_qb_select_listen_port'
assert_contains qnatter/files/qnatter-notify 'api/v2/auth/login'
assert_contains qnatter/files/qnatter-notify 'api/v2/app/preferences'
assert_contains qnatter/files/qnatter-notify 'api/v2/app/setPreferences'
assert_contains qnatter/files/qnatter-notify 'qBittorrent current listen_port'
assert_contains qnatter/files/qnatter-notify 'qBittorrent listen_port changed'
assert_contains qnatter/files/qnatter-notify 'source=\$listen_port_source mapping='
assert_not_contains qnatter/Makefile 'DEPENDS:=.*\+python3'
assert_contains qnatter/Makefile 'DEPENDS:=.*\+curl'
assert_contains qnatter/Makefile 'DEPENDS:=.*\+nftables'
assert_contains qnatter/Makefile 'DEPENDS:=.*\+firewall4'
assert_contains qnatter/Makefile 'DEPENDS:=.*\+kmod-nft-nat'
assert_contains qnatter/Makefile 'QNATTER_GOOS\?=linux'
assert_contains qnatter/Makefile 'QNATTER_GOARCH\?=\$\(GO_ARCH\)'
assert_contains qnatter/Makefile 'QNATTER_GOMIPS\?='
assert_contains qnatter/Makefile 'QNATTER_GOARM\?='
assert_contains qnatter/Makefile 'GOOS="\$\(QNATTER_GOOS\)"'
assert_contains qnatter/Makefile 'GOARCH="\$\(QNATTER_GOARCH\)"'
assert_contains qnatter/Makefile 'GOMIPS="\$\(QNATTER_GOMIPS\)"'
assert_contains qnatter/Makefile 'GOARM="\$\(QNATTER_GOARM\)"'
assert_contains qnatter/Makefile 'go build .* -o \$\(PKG_BUILD_DIR\)/qnatter-go ./cmd/qnatter'
assert_contains qnatter/Makefile '\$\(INSTALL_BIN\) \$\(PKG_BUILD_DIR\)/qnatter-go \$\(1\)/usr/bin/qnatter-go'
assert_contains qnatter/Makefile '\$\(LN\) qnatter-go \$\(1\)/usr/bin/QNatter'
qnatter_release="$(sed -n 's/^PKG_RELEASE:=//p' "$ROOT/qnatter/Makefile")"
[ "$qnatter_release" -ge 29 ] || fail "qnatter package release must increase when package files change"
assert_contains qnatter/Makefile '\$\(INSTALL_CONF\) ./files/qnatter.config \$\(1\)/etc/config/qnatter.default'
assert_contains qnatter/Makefile '\$\(INSTALL_DIR\) \$\(1\)/etc/uci-defaults'
assert_contains qnatter/Makefile '\$\(INSTALL_BIN\) ./files/qnatter.uci-default \$\(1\)/etc/uci-defaults/99-qnatter'
assert_not_contains qnatter/Makefile '\$\(INSTALL_CONF\) ./files/qnatter.config \$\(1\)/etc/config/qnatter$'
assert_not_contains qnatter/Makefile '\+iptables-nft'
assert_not_contains qnatter/Makefile '\+socat'
assert_not_contains qnatter/Makefile '\+gost'
assert_not_contains qnatter/Makefile '\+python3-light'
assert_not_contains qnatter/Makefile 'qnatter.py'
assert_not_contains qnatter/Makefile './files/QNatter'
assert_not_contains qnatter/Makefile 'qnatter-python-wrapper.py'
assert_contains luci-app-qnatter/Makefile '^PKG_RELEASE:=1$'
assert_contains luci-app-qnatter/Makefile 'LUCI_DEPENDS:=.*\+qnatter'
assert_contains luci-app-qnatter/Makefile 'LUCI_DEPENDS:=.*\+luci-base'
assert_contains luci-app-qnatter/Makefile 'LUCI_DEPENDS:=.*\+rpcd'
assert_contains luci-app-qnatter/Makefile 'LUCI_DEPENDS:=.*\+uhttpd'
assert_contains luci-app-qnatter/Makefile 'LUCI_DEPENDS:=.*\+uhttpd-mod-ubus'
assert_contains luci-app-qnatter/Makefile 'LUCI_DEPENDS:=.*\+uci'
assert_contains luci-app-qnatter/Makefile 'LUCI_DEPENDS:=.*\+jsonfilter'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'getCloudflareToken'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'findCloudflareOption'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'contextOption\.section\.children'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'refreshCloudflareZones'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'refreshCloudflareSrvRecords'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "hideInGrid\\(s\\.option\\(form\\.DummyValue, '_cloudflare_load_zones', _\\('Read zones'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "hideInGrid\\(s\\.option\\(form\\.DummyValue, '_cloudflare_load_records', _\\('Read SRV records'\\)"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "form\\.Button, '_cloudflare_load_zones'"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js "form\\.Button, '_cloudflare_load_records'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'cloudflareButtonRenderer\(refreshCloudflareZones\)'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'cloudflareButtonRenderer\(refreshCloudflareSrvRecords\)'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'return handler\(this, section_id\)'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'var tokenOption, zoneOption, recordOption'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'o\.load = function\(section_id\)[^}]*callCloudflareZones'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'o\.load = function\(section_id\)[^}]*callCloudflareSrvRecords'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js 'return refreshCloudflareSrvRecords\(contextOption, section_id\);'
assert_option_block_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js cloudflare_api_token "refreshCloudflareZones(this, section_id)"
assert_option_block_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js cloudflare_zone_id "refreshCloudflareSrvRecords(this, section_id)"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js _cloudflare_load_zones "o.inputtitle = _('Read zones')"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js _cloudflare_load_zones "o.inputstyle = 'apply'"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js _cloudflare_load_zones "o.default = '1'"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js _cloudflare_load_zones "o.rmempty = true"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js _cloudflare_load_zones "o.renderWidget = cloudflareButtonRenderer(refreshCloudflareZones)"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js _cloudflare_load_records "o.inputtitle = _('Read SRV records')"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js _cloudflare_load_records "o.inputstyle = 'apply'"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js _cloudflare_load_records "o.default = '1'"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js _cloudflare_load_records "o.rmempty = true"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances.js _cloudflare_load_records "o.renderWidget = cloudflareButtonRenderer(refreshCloudflareSrvRecords)"
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter 'CLOUDFLARE_TIMEOUT:-4'
assert_contains qnatter/files/qnatter.uci-default 'QNATTER_UCI_CONFIG:=/etc/config/qnatter'
assert_contains qnatter/files/qnatter.uci-default 'QNATTER_UCI_DEFAULT:=/etc/config/qnatter.default'
assert_contains qnatter/files/qnatter.uci-default 'QNATTER_UCI_BIN:=uci'
assert_contains qnatter/files/qnatter.uci-default 'qnatter.global.hot_reload'
assert_contains qnatter/files/qnatter.uci-default 'qnatter_migrate_route_slot'
assert_contains qnatter/files/qnatter.uci-default 'route_slot'
assert_contains qnatter/files/qnatter.uci-default 'delete "qnatter\.\$\{section\}\.label"'
assert_contains qnatter/files/qnatter.uci-default 'cp "\$QNATTER_UCI_DEFAULT" "\$QNATTER_UCI_CONFIG"'

assert_po_translation 'Global Settings' '全局设置'
assert_po_translation 'Hot reload' '热加载'
assert_po_translation 'Reload notify-only changes without restarting running QNatter processes.' '仅重载通知类配置变更，不重启正在运行的 QNatter 进程。'
assert_po_translation 'Expose ports behind full-cone NAT with optional forwarding and qBittorrent port updates.' '通过全锥形 NAT 暴露端口，并可选转发和更新 qBittorrent 监听端口。'
assert_po_translation 'Runtime' '运行时'
assert_not_contains luci-app-qnatter/po/zh_Hans/qnatter.po 'msgid "Label"'
assert_po_translation 'WAN interface' 'WAN 接口'
assert_po_translation 'Leave empty to bind to the default WAN device.' '留空则自动绑定默认 WAN 设备。'
assert_po_translation 'Forward method' '转发方式'
assert_po_translation 'Auto firewall' '自动防火墙'
assert_po_translation 'Automatically opens this instance current QNatter port on the WAN firewall.' '自动在 WAN 防火墙上放行此实例当前的 QNatter 端口。'
assert_po_translation 'Instance ID' '实例 ID'
assert_po_translation 'letters, numbers, and underscore' '字母、数字和下划线'
assert_po_translation 'Rename instance' '重命名实例'
assert_po_translation 'Rename failed' '重命名失败'
assert_po_translation 'Invalid new instance name' '实例 ID 无效'
assert_po_translation 'Instance name already exists' '实例 ID 已存在'
assert_po_translation 'Forward target port' '转发目标端口'
assert_po_translation 'Custom' '自定义'
assert_po_translation 'STUN server' 'STUN 服务器'
assert_po_translation 'Try these instance STUN servers before the global list.' '优先尝试这些实例 STUN 服务器，然后继续尝试全局列表。'
assert_po_translation 'Cloudflare SRV' 'Cloudflare SRV'
assert_po_translation 'Cloudflare API token/key' 'Cloudflare API Token/Key'
assert_po_translation 'Read zones' '读取区域'
assert_po_translation 'Cloudflare zone' 'Cloudflare 区域'
assert_po_translation 'Cloudflare SRV record' 'Cloudflare SRV 记录'
assert_po_translation 'Read SRV records' '读取 SRV 记录'
assert_not_contains luci-app-qnatter/po/zh_Hans/qnatter.po 'Port 0 forwards to the QNatter mapped internal port\.'
assert_po_translation 'Port 0 forwards to the port QNatter reports after punching.' '端口 0 会转发到 QNatter 打洞后报告的端口。'
assert_po_translation 'QNatter Status' 'QNatter 状态'
assert_po_translation 'Public address' '公网地址'
assert_po_translation 'Internal address' '内部地址'
assert_po_translation 'Waiting for mapping' '等待映射'
assert_po_translation 'QNatter Logs' 'QNatter 日志'
assert_po_translation 'Clear logs' '清空日志'
assert_po_translation 'Logs cleared' '日志已清空'

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
cp "$ROOT/qnatter/files/qnatter-qbittorrent.sh" "$tmp/"

(
	. "$tmp/qnatter-qbittorrent.sh"
	[ "$(qnatter_qb_select_listen_port 5000 62000 0)" = "62000" ] || exit 11
	[ "$(qnatter_qb_select_listen_port 5000 62000 51413)" = "51413" ] || exit 12
	qb_port_err="$tmp/qb-port.err"
	[ "$(qnatter_qb_select_listen_port 5000 62000 999999999999999999999999 2>"$qb_port_err")" = "62000" ] || exit 19
	[ ! -s "$qb_port_err" ] || exit 20
	[ "$(qnatter_qb_preferences_json 62000)" = '{"listen_port":62000}' ] || exit 13
	[ "$(qnatter_qb_normalize_url 'http://127.0.0.1:8080///')" = "http://127.0.0.1:8080" ] || exit 18

	env_file="$tmp/qb-env"
	qnatter_qb_write_notify_env "$env_file" \
		QBITTORRENT_URL "http://127.0.0.1:8080/path with spaces" \
		QBITTORRENT_USERNAME "admin user" \
		QBITTORRENT_PASSWORD "pa'ss word"
	# shellcheck disable=SC1090
	. "$env_file"
	[ "$QBITTORRENT_URL" = "http://127.0.0.1:8080/path with spaces" ] || exit 14
	[ "$QBITTORRENT_USERNAME" = "admin user" ] || exit 15
	[ "$QBITTORRENT_PASSWORD" = "pa'ss word" ] || exit 16

	chmod 0644 "$env_file"
	qnatter_qb_write_notify_env "$env_file" QBITTORRENT_PASSWORD "secret"
	mode="$(ls -l "$env_file" | awk '{ print $1 }')"
	case "$mode" in
		-rw-------) : ;;
		*) exit 17 ;;
	esac

	previous_umask="$(umask)"
	umask 022
	qnatter_qb_write_notify_env "$env_file" QBITTORRENT_PASSWORD "secret"
	[ "$(umask)" = "0022" ] || exit 19
	umask "$previous_umask"

	failing_env="$tmp/qb-env-failing"
	failing_tmp="${failing_env}.$$"
	mv() { return 1; }
	if qnatter_qb_write_notify_env "$failing_env" QBITTORRENT_PASSWORD "secret"; then
		exit 33
	fi
	unset -f mv
	[ ! -e "$failing_tmp" ] || exit 34

	odd_env="$tmp/qb-env-odd"
	if qnatter_qb_write_notify_env "$odd_env" QBITTORRENT_PASSWORD; then
		exit 35
	fi
	[ ! -e "$odd_env" ] || exit 36
	[ ! -e "${odd_env}.$$" ] || exit 37
)

(
	. "$ROOT/qnatter/files/qnatter-common.sh"
	old_path="$PATH"

	[ "$(qnatter_slug wan_ct)" = "wan_ct" ] || exit 26
	[ "$(qnatter_slug 'wan-ct')" = "wan_ct" ] || exit 27
	[ "$(qnatter_runtime_slug wan_ct)" = "wan_ct" ] || exit 29
	[ "$(qnatter_runtime_slug 'wan-ct')" != "$(qnatter_runtime_slug wan_ct)" ] || exit 30
	[ "$(qnatter_runtime_slug 'wan.ct')" != "$(qnatter_runtime_slug wan_ct)" ] || exit 31
	[ "$(qnatter_runtime_slug 'wan_x2dct')" != "$(qnatter_runtime_slug 'wan-ct')" ] || exit 32
	json_input="$(printf 'line "one"\\\nline\rtwo\tend')"
	[ "$(qnatter_json_escape "$json_input")" = 'line \"one\"\\\nline\rtwo\tend' ] || exit 28
	status_json="$tmp/invalid-port-status.json"
	qnatter_write_status_json "$status_json" wan tcp 192.0.2.10 not-a-port 198.51.100.10 999999999999999999999 mapped
	grep -Fq '"inner_port":0' "$status_json" || exit 38
	grep -Fq '"outer_port":0' "$status_json" || exit 39
	atomic_status="$tmp/atomic-status.json"
	atomic_link="$tmp/atomic-status-old.json"
	printf 'old status\n' > "$atomic_status"
	ln "$atomic_status" "$atomic_link" || exit 40
	qnatter_write_status_json "$atomic_status" wan tcp 192.0.2.10 51413 198.51.100.10 62000 mapped
	[ "$(cat "$atomic_link")" = "old status" ] || exit 41

	QNATTER_FORWARD_METHOD=iptables
	qnatter_build_args
	case "$QNATTER_ARGS" in *iptables*|*'-m'*) exit 21 ;; esac

	PATH=/nonexistent QNATTER_FORWARD_METHOD=socat
	qnatter_build_args
	case "$QNATTER_ARGS" in *socat*|*'-m'*) exit 22 ;; esac

	PATH=/nonexistent QNATTER_FORWARD_METHOD=gost
	qnatter_build_args
	case "$QNATTER_ARGS" in *gost*|*'-m'*) exit 24 ;; esac
	PATH="$old_path"

	mkdir -p "$tmp/bin"
	printf '#!/bin/sh\n' > "$tmp/bin/socat"
	chmod 0755 "$tmp/bin/socat"
	printf '#!/bin/sh\n' > "$tmp/bin/gost"
	chmod 0755 "$tmp/bin/gost"

	PATH="$tmp/bin" QNATTER_FORWARD_METHOD=socat
	qnatter_build_args
	case "$QNATTER_ARGS" in *socat*) ;; *) exit 23 ;; esac

	PATH="$tmp/bin" QNATTER_FORWARD_METHOD=gost
	qnatter_build_args
	case "$QNATTER_ARGS" in *gost*) ;; *) exit 25 ;; esac
)

sh -n "$ROOT/qnatter/files/qnatter-common.sh"
sh -n "$ROOT/qnatter/files/qnatter-qbittorrent.sh"
sh -n "$ROOT/qnatter/files/qnatter-notify"
sh -n "$ROOT/qnatter/files/qnatter-run"
sh -n "$ROOT/qnatter/files/qnatter.init"
sh -n "$ROOT/tests/test_qnatter_hotplug.sh"
sh -n "$ROOT/luci-app-qnatter/root/usr/libexec/qnatter-status"
sh -n "$ROOT/luci-app-qnatter/root/usr/libexec/qnatter-log"
sh -n "$ROOT/luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter"
sh -n "$ROOT/tests/test_qnatter_init.sh"
sh -n "$ROOT/tests/test_qnatter_log.sh"
sh -n "$ROOT/tests/test_qnatter_notify.sh"
sh -n "$ROOT/tests/test_qnatter_rpcd.sh"
sh -n "$ROOT/tests/test_qnatter_status.sh"
sh -n "$ROOT/tests/test_qnatter_uci_default.sh"

(
	printf '#!/bin/sh\necho "go:$*"\n' > "$tmp/qnatter-go-bin"
	chmod 0755 "$tmp/qnatter-go-bin"

	QNATTER_GO_BIN="$tmp/qnatter-go-bin" \
		"$ROOT/qnatter/files/qnatter-run" "$tmp/default-runtime.log" alpha beta
	grep -qx 'go:alpha beta' "$tmp/default-runtime.log" || fail "qnatter-run default runtime must execute qnatter-go"

	QNATTER_RUNTIME=python QNATTER_GO_BIN="$tmp/qnatter-go-bin" \
		"$ROOT/qnatter/files/qnatter-run" "$tmp/python-runtime.log" gamma
	grep -qx 'go:gamma' "$tmp/python-runtime.log" || fail "qnatter-run legacy python runtime must execute qnatter-go"
)

(cd "$ROOT/go-qnatter" && go test ./...)
"$ROOT/tests/test_qnatter_hotplug.sh"
"$ROOT/tests/test_qnatter_init.sh"
"$ROOT/tests/test_qnatter_log.sh"
"$ROOT/tests/test_qnatter_notify.sh"
"$ROOT/tests/test_qnatter_rpcd.sh"
"$ROOT/tests/test_qnatter_status.sh"
"$ROOT/tests/test_qnatter_uci_default.sh"

printf 'qnatter package static checks passed\n'
