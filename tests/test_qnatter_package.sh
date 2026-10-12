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

assert_line_order() {
	file="$1"
	first="$2"
	second="$3"
	awk -v first="$first" -v second="$second" '
		index($0, first) && first_line == 0 {
			first_line = NR
		}
		index($0, second) && second_line == 0 {
			second_line = NR
		}
		END {
			exit(first_line > 0 && second_line > 0 && first_line < second_line ? 0 : 1)
		}
	' "$ROOT/$file" || fail "$file must place '$first' before '$second'"
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

assert_file LICENSE
assert_contains LICENSE 'GNU GENERAL PUBLIC LICENSE'
assert_contains LICENSE 'Version 3, 29 June 2007'
assert_contains qnatter/Makefile '^# SPDX-License-Identifier: GPL-3\.0-only$'
assert_contains qnatter/Makefile '^PKG_LICENSE:=GPL-3\.0-only$'
assert_contains qnatter/Makefile '^PKG_LICENSE_FILES:=LICENSE$'
assert_contains luci-app-qnatter/Makefile '^# SPDX-License-Identifier: GPL-3\.0-only$'
assert_contains luci-app-qnatter/Makefile '^PKG_LICENSE:=GPL-3\.0-only$'
assert_contains luci-app-qnatter/Makefile '^PKG_LICENSE_FILES:=LICENSE$'
assert_contains README.md '^## 许可证$'
assert_contains README.md 'GPL-3\.0-only'

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
assert_contains luci-app-qnatter/Makefile '^PKG_RELEASE:=2$'
assert_contains luci-app-qnatter/Makefile 'define Package/luci-app-qnatter/postinst'
assert_contains luci-app-qnatter/Makefile 'chown root:root "\$\$root\$\$path"'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "^'require form';"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "^'require fs';"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "^'require rpc';"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "^'require tools\\.widgets as widgets';"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'cbi\('
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'qnatter-theme-aurora'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "L\\.resource\\('qnatter/qnatter\\.css'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "\\?v=1\\.1\\.0-r1"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/stun.js "\\?v=1\\.1\\.0-r1"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'hideInGrid'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'option\.keylist'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'indexOf\(String\(value\)\)'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "return E\\('button'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "hideInGrid\\(s\\.option\\(widgets\\.DeviceSelect, 'bind_value'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "widgets\\.DeviceSelect, 'bind_value', _\\('WAN interface'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "form\\.Flag, 'hot_reload', _\\('Hot reload'\\)"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "form\\.Value, 'label'"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "form\\.ListValue, 'runtime'"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "/usr/bin/python3"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'tools\.python'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "o\\.value\\('python'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'o\.rmempty = true'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'o\.nocreate = true'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "s\\.option\\(form\\.ListValue, 'network'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "fs\\.stat\\('/usr/bin/socat'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "fs\\.stat\\('/usr/bin/gost'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'tools\.socat'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'tools\.gost'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "o\\.value\\('iptables'"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "o\\.value\\('iptables-snat'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "o\\.depends\\('qbittorrent_forward', '0'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "o\\.depends\\('qbittorrent_enabled', '0'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "hideInGrid\\(s\\.option\\(form\\.Flag, 'auto_firewall'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "hideInGrid\\(s\\.option\\(form\\.Value, 'target_ip'"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js target_ip "o.depends('qbittorrent_enabled', '0')"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js target_ip "o.depends('qbittorrent_forward', '0')"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js target_port "o.depends('qbittorrent_enabled', '0')"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js target_port "o.depends('qbittorrent_forward', '0')"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'Port 0 forwards to the QNatter mapped internal port\.'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "form\\.DynamicList, 'stun_server'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "renderStunDynamicList"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "filterListValues"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "refreshDynamicListChoices"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "o\\.placeholder = _\\('Custom'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "Try these instance STUN servers before the global list"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "custom_placeholder = 'host\\[:port\\]'"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "Select a candidate or enter a custom host\\[:port\\]"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "uniqueListValues"
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
assert_contains luci-app-qnatter/root/usr/share/rpcd/acl.d/luci-app-qnatter.json '"completion_webhook_test"'
assert_contains luci-app-qnatter/root/usr/share/rpcd/acl.d/luci-app-qnatter.json '"completion_script_test"'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "form\\.Value, 'notify_script'"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'addOptionClass'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'qnatter-automation-card-start'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'qnatter-automation-card'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'qnatter-automation-card'
assert_line_order luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "s.option(form.ListValue, 'qbittorrent_forward_method'" "s.option(form.DummyValue, '_automation_panel'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "qnatter-automation-title"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "layout=automation6"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "s\\.option\\(form\\.DummyValue, '_automation_panel'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'renderAutomationPanel'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'writeAutomationField'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'callCompletionWebhookTest'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'callCompletionScriptTest'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'Script manual trigger test'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'setAutomationResult'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'Running script test...'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'Running webhook test...'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "data-result': 'script-test"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "data-result': 'webhook-test"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "ui\\.addNotification\\(null, E\\('p', \\[ text \\]\\), ok \\? 'info' : 'danger'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'completion_webhook_success_check'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'completion_webhook_method'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'completion_webhook_headers'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'completion_webhook_body'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'completion_webhook_success'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'completion_webhook_disable_success_check'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'completion_webhook_skip_unchanged'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'completion_script_inline'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'qnatter-automation-panel'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'qnatter-automation-section'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'qnatter-automation-grid'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'qnatter-automation-result'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'qnatter-automation-result.is-running'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'qnatter-automation-result.is-success'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'qnatter-automation-result.is-error'
assert_css_block_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '.qnatter-automation-panel' 'width: 100%'
assert_css_block_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '.qnatter-form-page .cbi-value[data-name="_automation_panel"] > .cbi-value-title' 'display: none'
assert_css_block_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '.qnatter-form-page .cbi-value[data-name="_automation_panel"] > .cbi-value-field' 'width: 100%'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "hideInGrid\\(s\\.option\\(form\\.Flag, 'cloudflare_enabled'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "hideInGrid\\(s\\.option\\(form\\.Value, 'cloudflare_api_token'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "hideInGrid\\(s\\.option\\(form\\.ListValue, 'cloudflare_zone_id'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "hideInGrid\\(s\\.option\\(form\\.ListValue, 'cloudflare_record_id'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'callCloudflareZones'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'callCloudflareSrvRecords'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'callRenameInstance'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "rename_instance"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "form\\.Value, '_rename_to'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "form\\.DummyValue, '_rename_instance'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "_\\(result && result\\.error \\? result\\.error : 'Rename failed'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "hideInGrid\\(s\\.option\\(form\\.Flag, 'qbittorrent_enabled'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'qnatter-theme-aurora'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js "expect: \\{ '': \\{ instances: \\[\\] \\} \\}"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'data\.instances'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js "method: 'toggle_instance'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js "callToggleInstance\\(name\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js "toggleInstance\\(name, this\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js "_\\('RUNNING'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js "_\\('ABNORMAL'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'itemState'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'is-error'
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
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter 'qnatter\.global\.enabled'
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter 'set "qnatter\.global\.enabled=1"'
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter 'service enabled \(was: disabled\)'
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter 'valid_section_name "\$instance"'
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter '\$QNATTER_INIT_BIN" reload'
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter 'cloudflare_api_get'
assert_contains luci-app-qnatter/root/usr/libexec/rpcd/luci.qnatter 'dns_records\?type=SRV'
assert_contains luci-app-qnatter/root/usr/share/rpcd/acl.d/luci-app-qnatter.json '"cloudflare_zones"'
assert_contains luci-app-qnatter/root/usr/share/rpcd/acl.d/luci-app-qnatter.json '"cloudflare_srv_records"'
assert_contains luci-app-qnatter/root/usr/share/rpcd/acl.d/luci-app-qnatter.json '"rename_instance"'
assert_contains luci-app-qnatter/root/usr/share/rpcd/acl.d/luci-app-qnatter.json '"toggle_instance"'
assert_contains luci-app-qnatter/root/usr/libexec/qnatter-status 'grep -Fx'
assert_contains luci-app-qnatter/root/usr/libexec/qnatter-status 'json_pair state'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'theme-argon'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css 'qnatter-theme-aurora'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '\.qnatter-pill\.is-error'
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
assert_contains qnatter/files/qnatter.init 'qnatter_record_interface_trigger "\$network"'
assert_not_contains qnatter/files/qnatter.init '\[ -n "\$bind_value" \] && return 0'
assert_contains qnatter/files/qnatter-run 'rm -f "\$QNATTER_STATUS_FILE"'
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
assert_not_contains qnatter/files/qnatter.config "option notify_script"
assert_contains qnatter/files/qnatter.config "option completion_webhook_enabled '0'"
assert_contains qnatter/files/qnatter.config "option completion_webhook_url ''"
assert_contains qnatter/files/qnatter.config "option completion_webhook_timeout '8'"
assert_contains qnatter/files/qnatter.config "option completion_webhook_method 'POST'"
assert_contains qnatter/files/qnatter.config "option completion_webhook_headers 'Content-Type: application/json'"
assert_contains qnatter/files/qnatter.config "option completion_webhook_body"
assert_contains qnatter/files/qnatter.config "option completion_webhook_success ''"
assert_contains qnatter/files/qnatter.config "option completion_webhook_disable_success_check '1'"
assert_contains qnatter/files/qnatter.config "option completion_webhook_skip_unchanged '0'"
assert_contains qnatter/files/qnatter.config "option completion_script_enabled '0'"
assert_contains qnatter/files/qnatter.config "option completion_script_inline ''"
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
assert_not_contains qnatter/files/qnatter.init 'QNATTER_USER_NOTIFY'
assert_not_contains qnatter/files/qnatter.init 'QNATTER_COMPLETION_SCRIPT_FILE'
assert_contains qnatter/files/qnatter.init 'QNATTER_COMPLETION_WEBHOOK_ENABLED'
assert_contains qnatter/files/qnatter.init 'QNATTER_COMPLETION_WEBHOOK_URL'
assert_contains qnatter/files/qnatter.init 'QNATTER_COMPLETION_WEBHOOK_METHOD'
assert_contains qnatter/files/qnatter.init 'QNATTER_COMPLETION_WEBHOOK_HEADERS'
assert_contains qnatter/files/qnatter.init 'QNATTER_COMPLETION_WEBHOOK_BODY'
assert_contains qnatter/files/qnatter.init 'QNATTER_COMPLETION_WEBHOOK_SUCCESS'
assert_contains qnatter/files/qnatter.init 'QNATTER_COMPLETION_WEBHOOK_DISABLE_SUCCESS_CHECK'
assert_contains qnatter/files/qnatter.init 'QNATTER_COMPLETION_WEBHOOK_SKIP_UNCHANGED'
assert_contains qnatter/files/qnatter.init 'QNATTER_COMPLETION_SCRIPT_ENABLED'
assert_contains qnatter/files/qnatter.init 'QNATTER_COMPLETION_SCRIPT_INLINE'
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
assert_contains qnatter/files/qnatter-notify 'run_completion_webhook'
assert_contains qnatter/files/qnatter-notify 'run_completion_script'
assert_contains qnatter/files/qnatter-notify 'QNATTER_SCRIPT_INSTANCE'
assert_not_contains qnatter/files/qnatter-notify 'QNATTER_USER_NOTIFY'
assert_not_contains qnatter/files/qnatter-notify 'QNATTER_COMPLETION_SCRIPT_FILE'
assert_contains qnatter/files/qnatter-notify 'QNATTER_COMPLETION_WEBHOOK_ENABLED'
assert_contains qnatter/files/qnatter-notify 'QNATTER_COMPLETION_WEBHOOK_URL'
assert_contains qnatter/files/qnatter-notify 'QNATTER_COMPLETION_WEBHOOK_METHOD'
assert_contains qnatter/files/qnatter-notify 'QNATTER_COMPLETION_WEBHOOK_HEADERS'
assert_contains qnatter/files/qnatter-notify 'QNATTER_COMPLETION_WEBHOOK_BODY'
assert_contains qnatter/files/qnatter-notify 'QNATTER_COMPLETION_WEBHOOK_SUCCESS'
assert_contains qnatter/files/qnatter-notify 'QNATTER_COMPLETION_WEBHOOK_DISABLE_SUCCESS_CHECK'
assert_contains qnatter/files/qnatter-notify 'QNATTER_COMPLETION_WEBHOOK_SKIP_UNCHANGED'
assert_contains qnatter/files/qnatter-notify 'qnatter_render_template'
assert_contains qnatter/files/qnatter-notify 'QNATTER_COMPLETION_SCRIPT_ENABLED'
assert_contains qnatter/files/qnatter-notify 'QNATTER_COMPLETION_SCRIPT_INLINE'
assert_contains qnatter/files/qnatter-notify 'QNATTER_LOG_FILE'
assert_not_contains qnatter/files/qnatter.config 'option runtime'
assert_not_contains qnatter/files/qnatter.init 'config_get runtime'
assert_not_contains qnatter/files/qnatter.init 'PROG="/usr/bin/qnatter.py"'
assert_not_contains qnatter/files/qnatter.init 'mapped internal port'
assert_contains qnatter/files/qnatter.init 'qbittorrent_target_ip'
assert_not_contains qnatter/files/qnatter-run 'QNATTER_PY_BIN'
assert_not_contains qnatter/files/qnatter-run 'QNATTER_RUNTIME'
assert_contains qnatter/files/qnatter-run 'QNATTER_GO_BIN:-/usr/bin/qnatter-go'
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
assert_contains qnatter/Makefile 'QNATTER_GOOS:=linux'
assert_contains qnatter/Makefile 'QNATTER_GOARCH:=\$\(subst aarch64,arm64'
assert_contains qnatter/Makefile 'x86_64,amd64,\$\(ARCH\)'
assert_contains qnatter/Makefile 'QNATTER_GOMIPS:='
assert_contains qnatter/Makefile 'QNATTER_GOARM:='
assert_contains qnatter/Makefile 'QNATTER_GOMIPS64:='
# Mapping variables must not be overridable from the environment (XB-01 / N1).
assert_not_contains qnatter/Makefile 'QNATTER_GOARCH\?='
assert_not_contains qnatter/Makefile 'QNATTER_GOARM\?='
assert_not_contains qnatter/Makefile 'QNATTER_GOMIPS\?='
assert_not_contains qnatter/Makefile 'QNATTER_GO386\?='
assert_contains qnatter/Makefile 'QNATTER_EXPECTED_GOARCH:=\$\(subst aarch64,arm64'
assert_contains qnatter/Makefile 'vfp vfpv2,\$\(QNATTER_TARGET_FPU\)'
assert_contains qnatter/Makefile 'CONFIG_HAS_FPU'
assert_contains qnatter/Makefile 'cannot map target architecture'
assert_contains qnatter/Makefile 'GOOS="\$\(QNATTER_GOOS\)"'
assert_contains qnatter/Makefile 'GOARCH="\$\(QNATTER_GOARCH\)"'
assert_contains qnatter/Makefile 'GOMIPS="\$\(QNATTER_GOMIPS\)"'
assert_contains qnatter/Makefile 'GOMIPS64="\$\(QNATTER_GOMIPS64\)"'
assert_contains qnatter/Makefile 'GOARM="\$\(QNATTER_GOARM\)"'
assert_contains qnatter/Makefile 'QNATTER_GO386:='
assert_contains qnatter/Makefile 'CONFIG_TARGET_x86_geode'
assert_contains qnatter/Makefile 'GO386="\$\(QNATTER_GO386\)"'
assert_contains qnatter/Makefile 'QNATTER_SUPPORTED_GOARCH:='
assert_contains qnatter/Makefile 'unsupported GOARCH'
assert_contains qnatter/Makefile 'build -buildvcs=false'
# 自校验必须放在独立脚本里：define 里的 `$$` 要经过两轮 make 展开才到达 shell，
# 内联的 $$bin 会被吃成 `in`、$$(go version -m ...) 变成空串，于是完全正确的
# 构建反而报出 "GOARCH=uilt"。
assert_file qnatter/scripts/check-goarch.sh
assert_contains qnatter/Makefile 'sh \$\(CURDIR\)/scripts/check-goarch\.sh "\$\(PKG_BUILD_DIR\)/qnatter-go"'
assert_contains qnatter/Makefile '"\$\(QNATTER_EXPECTED_GOARCH\)" "\$\(ARCH\)"'
assert_not_contains qnatter/Makefile '\$\$bin'
assert_not_contains qnatter/Makefile '\$\$built'
assert_not_contains qnatter/Makefile '\$\$out'
assert_contains qnatter/scripts/check-goarch.sh 'GOARCH=\$built, expected \$expected'
assert_contains qnatter/scripts/check-goarch.sh 'cannot determine the GOARCH'
assert_contains qnatter/scripts/check-goarch.sh 'go version -m "\$binary"'
assert_not_contains qnatter/scripts/check-goarch.sh 'grep -o .GOARCH='
assert_not_contains qnatter/scripts/check-goarch.sh '\$\$'
assert_contains qnatter/Makefile 'DEPENDS:=.*@\(aarch64||arm||'
assert_contains qnatter/Makefile 'go build .* -o \$\(PKG_BUILD_DIR\)/qnatter-go ./cmd/qnatter'
# XB-07: sub-architecture baselines are pinned explicitly.
assert_contains qnatter/Makefile 'QNATTER_GOAMD64:='
assert_contains qnatter/Makefile 'QNATTER_GOARM64:='
assert_contains qnatter/Makefile 'QNATTER_GO_PPC64:='
assert_contains qnatter/Makefile 'GOAMD64="\$\(QNATTER_GOAMD64\)"'
assert_contains qnatter/Makefile 'GOARM64="\$\(QNATTER_GOARM64\)"'
assert_contains qnatter/Makefile 'GO_PPC64="\$\(QNATTER_GO_PPC64\)"'
# XB-06: no upstream tarball is downloaded (the Go sources ship in-repo), so
# PKG_SOURCE/PKG_MIRROR_HASH must not reappear with an unchecked download.
assert_contains qnatter/Makefile 'define Build/Prepare'
assert_not_contains qnatter/Makefile '^PKG_SOURCE'
assert_not_contains qnatter/Makefile '^PKG_MIRROR_HASH'
assert_not_contains qnatter/Makefile 'MikeWang000000/Natter.git'
# XB-06 follow-up: Build/Prepare must not assume a LICENSE exists one level up.
# In the feeds layout the repository root is that parent (feeds/qnatter/LICENSE),
# but a copy dropped straight into package/qnatter has package/ above it, which
# holds no LICENSE - the unconditional "$(CP) $(CURDIR)/../LICENSE" failed the
# whole build there. The in-package copy must win, and a missing LICENSE must
# never be fatal.
assert_file qnatter/LICENSE
assert_contains qnatter/LICENSE 'GNU GENERAL PUBLIC LICENSE'
assert_contains qnatter/Makefile '\$\(CURDIR\)/LICENSE'
assert_contains qnatter/Makefile '\$\(CURDIR\)/\.\./LICENSE'
assert_not_contains qnatter/Makefile '^\t\$\(CP\) \$\(CURDIR\)/\.\./LICENSE'

# Layout simulation: run the real Build/Prepare recipe body in every layout a
# user can hit, and make sure a missing LICENSE can no longer abort the build.
if command -v make >/dev/null 2>&1; then
	bp_tmp="$(mktemp -d)"
	awk '/^define Build\/Prepare$/{f=1} f{print} /^endef$/{if(f)exit}' \
		"$ROOT/qnatter/Makefile" >"$bp_tmp/def.mk"
	[ -s "$bp_tmp/def.mk" ] || fail 'could not extract Build/Prepare from qnatter/Makefile'

	# $1 name, $2 package dir, $3 own LICENSE (yes/no), $4 parent LICENSE (yes/no)
	bp_case() {
		bp_name="$1"
		bp_dir="$2"
		bp_out="$bp_tmp/out-$1"
		rm -rf "$bp_dir" "$bp_out"
		mkdir -p "$bp_dir"
		if [ "$3" = yes ]; then echo "OWN-$bp_name" >"$bp_dir/LICENSE"; fi
		if [ "$4" = yes ]; then echo "PARENT-$bp_name" >"$(dirname "$bp_dir")/LICENSE"; fi
		{
			echo "CURDIR := $bp_dir"
			echo "PKG_BUILD_DIR := $bp_out"
			echo "CP := cp -fpR"
			cat "$bp_tmp/def.mk"
			printf 'prepare:\n\t$(Build/Prepare)\n'
		} >"$bp_tmp/$bp_name.mk"
		make -s -f "$bp_tmp/$bp_name.mk" prepare >"$bp_tmp/$bp_name.log" 2>&1 ||
			fail "Build/Prepare failed in layout $bp_name: $(head -1 "$bp_tmp/$bp_name.log")"
	}

	bp_case feeds "$bp_tmp/feeds/qnatter/qnatter" no yes
	[ -f "$bp_tmp/out-feeds/LICENSE" ] ||
		fail 'Build/Prepare did not copy the parent LICENSE in the feeds layout'

	bp_case package "$bp_tmp/package/qnatter" yes no
	grep -q '^OWN-package$' "$bp_tmp/out-package/LICENSE" ||
		fail 'Build/Prepare did not prefer the in-package LICENSE'

	bp_case nolicense "$bp_tmp/nolicense/qnatter" no no

	rm -rf "$bp_tmp"
else
	echo 'SKIP: make not available; Build/Prepare layout simulation skipped' >&2
fi

# scripts/check-goarch.sh 是构建期唯一判断 GOARCH 的地方。用桩 `go` 直接跑它，
# 命中、不匹配、输出里没有 GOARCH 记录、期望值为空这四种情况都要有确定结果。
ga_tmp="$(mktemp -d)"
mkdir -p "$ga_tmp/bin"
: >"$ga_tmp/fake-binary"
ga_stub() {
	cat >"$ga_tmp/bin/go" <<STUB
#!/bin/sh
printf '$1\n'
STUB
	chmod +x "$ga_tmp/bin/go"
}
ga_run() {
	PATH="$ga_tmp/bin:$PATH" sh "$ROOT/qnatter/scripts/check-goarch.sh" \
		"$ga_tmp/fake-binary" "$2" "$3" >/dev/null 2>&1
}

ga_stub 'build\tGOARCH=amd64'
ga_run matching amd64 x86_64 || fail 'check-goarch.sh rejected a matching GOARCH'
if ga_run mismatched arm64 aarch64; then
	fail 'check-goarch.sh accepted a mismatched GOARCH'
fi

ga_stub 'build\tGOOS=linux'
if ga_run missing amd64 x86_64; then
	fail 'check-goarch.sh accepted output with no GOARCH record'
fi

ga_stub 'build\tGOARCH=amd64'
if ga_run emptyexpect '' x86_64; then
	fail 'check-goarch.sh accepted an empty expected GOARCH'
fi
rm -rf "$ga_tmp"

assert_contains qnatter/Makefile '\$\(INSTALL_BIN\) \$\(PKG_BUILD_DIR\)/qnatter-go \$\(1\)/usr/bin/qnatter-go'
assert_contains qnatter/Makefile '\$\(LN\) qnatter-go \$\(1\)/usr/bin/QNatter'
assert_contains qnatter/Makefile '^PKG_VERSION:=2\.2\.2$'
assert_contains qnatter/Makefile '^PKG_RELEASE:=2$'
assert_contains qnatter/Makefile '\$\(INSTALL_CONF\) ./files/qnatter.config \$\(1\)/etc/config/qnatter.default'
assert_contains qnatter/Makefile '\$\(INSTALL_DIR\) \$\(1\)/etc/uci-defaults'
assert_contains qnatter/Makefile '\$\(INSTALL_BIN\) ./files/qnatter.uci-default \$\(1\)/etc/uci-defaults/99-qnatter'
assert_contains qnatter/Makefile 'define Package/qnatter/postinst'
assert_contains qnatter/Makefile 'chown root:root "\$\$root\$\$path"'
assert_contains qnatter/Makefile 'chown -h root:root "\$\$root/usr/bin/QNatter"'
assert_not_contains qnatter/Makefile '\$\(INSTALL_CONF\) ./files/qnatter.config \$\(1\)/etc/config/qnatter$'
assert_not_contains qnatter/Makefile '\+iptables-nft'
assert_not_contains qnatter/Makefile '\+socat'
assert_not_contains qnatter/Makefile '\+gost'
assert_not_contains qnatter/Makefile '\+python3-light'
assert_not_contains qnatter/Makefile 'qnatter.py'
assert_not_contains qnatter/Makefile './files/QNatter'
assert_not_contains qnatter/Makefile 'qnatter-python-wrapper.py'
assert_contains luci-app-qnatter/Makefile 'LUCI_DEPENDS:=.*\+qnatter'
assert_contains luci-app-qnatter/Makefile 'LUCI_DEPENDS:=.*\+luci-base'
assert_contains luci-app-qnatter/Makefile 'LUCI_DEPENDS:=.*\+rpcd'
assert_contains luci-app-qnatter/Makefile 'LUCI_DEPENDS:=.*\+uhttpd'
assert_contains luci-app-qnatter/Makefile 'LUCI_DEPENDS:=.*\+uhttpd-mod-ubus'
assert_contains luci-app-qnatter/Makefile 'LUCI_DEPENDS:=.*\+uci'
assert_contains luci-app-qnatter/Makefile 'LUCI_DEPENDS:=.*\+jsonfilter'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'getCloudflareToken'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'findCloudflareOption'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'contextOption\.section\.children'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'refreshCloudflareZones'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'refreshCloudflareSrvRecords'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "hideInGrid\\(s\\.option\\(form\\.DummyValue, '_cloudflare_load_zones', _\\('Read zones'\\)"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "hideInGrid\\(s\\.option\\(form\\.DummyValue, '_cloudflare_load_records', _\\('Read SRV records'\\)"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "form\\.Button, '_cloudflare_load_zones'"
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js "form\\.Button, '_cloudflare_load_records'"
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'cloudflareButtonRenderer\(refreshCloudflareZones\)'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'cloudflareButtonRenderer\(refreshCloudflareSrvRecords\)'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'return handler\(this, section_id\)'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'var tokenOption, zoneOption, recordOption'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'o\.load = function\(section_id\)[^}]*callCloudflareZones'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'o\.load = function\(section_id\)[^}]*callCloudflareSrvRecords'
assert_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js 'return refreshCloudflareSrvRecords\(contextOption, section_id\);'
assert_option_block_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js cloudflare_api_token "refreshCloudflareZones(this, section_id)"
assert_option_block_not_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js cloudflare_zone_id "refreshCloudflareSrvRecords(this, section_id)"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js _cloudflare_load_zones "o.inputtitle = _('Read zones')"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js _cloudflare_load_zones "o.inputstyle = 'apply'"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js _cloudflare_load_zones "o.default = '1'"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js _cloudflare_load_zones "o.rmempty = true"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js _cloudflare_load_zones "o.renderWidget = cloudflareButtonRenderer(refreshCloudflareZones)"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js _cloudflare_load_records "o.inputtitle = _('Read SRV records')"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js _cloudflare_load_records "o.inputstyle = 'apply'"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js _cloudflare_load_records "o.default = '1'"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js _cloudflare_load_records "o.rmempty = true"
assert_option_block_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/instances-v11.js _cloudflare_load_records "o.renderWidget = cloudflareButtonRenderer(refreshCloudflareSrvRecords)"
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
assert_po_translation 'Custom script' '自定义脚本'
assert_not_contains luci-app-qnatter/po/zh_Hans/qnatter.po 'Port 0 forwards to the QNatter mapped internal port\.'
assert_po_translation 'Port 0 forwards to the port QNatter reports after punching.' '端口 0 会转发到 QNatter 打洞后报告的端口。'
assert_po_translation 'QNatter Status' 'QNatter 状态'
assert_po_translation 'Public address' '公网地址'
assert_po_translation 'Internal address' '内部地址'
assert_po_translation 'Copy' '复制'
assert_po_translation 'Copied' '已复制'
assert_po_translation 'Copy failed' '复制失败'
assert_po_translation 'Copy public address' '复制公网地址'
assert_po_translation 'Copy internal address' '复制内部地址'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'qnatter-address-field'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'qnatter-copy-button'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'qnatter-copy-icon'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/view/qnatter/status.js 'navigator\.clipboard\.writeText'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '\.qnatter-address-field'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '\.qnatter-copy-button\.is-copied'
assert_contains luci-app-qnatter/htdocs/luci-static/resources/qnatter/qnatter.css '\.qnatter-copy-label'
assert_po_translation 'Waiting for mapping' '等待映射'
assert_po_translation 'QNatter Logs' 'QNatter 日志'
assert_po_translation 'Clear logs' '清空日志'
assert_po_translation 'Logs cleared' '日志已清空'
assert_po_translation 'Loading...' '加载中…'
assert_po_translation 'Allow private webhook targets' '允许 Webhook 指向内网/回环地址'
assert_po_translation 'Allow completion webhooks to target loopback, private and link-local addresses. This disables the SSRF guard for webhook requests, so a webhook may reach services on this router or anywhere in the LAN; keep it off unless a trusted local receiver is required.' '允许完成通知 Webhook 指向回环、内网或链路本地地址。这会关闭 Webhook 请求的 SSRF 防护，Webhook 可能访问本机或局域网内的任意服务；除非确实需要可信的本地接收端，否则请保持关闭。'
assert_po_translation 'Private webhook targets are now allowed: completion webhooks may reach loopback, private and link-local addresses, including other services on this router or in the LAN. Save the form to apply this change.' '已允许 Webhook 指向内网/回环地址：完成通知 Webhook 现在可以访问回环、内网和链路本地地址（包括本路由器或局域网内的其他服务）。保存表单后生效。'
assert_po_translation 'Private webhook targets are blocked again: completion webhooks pointing at loopback, private or link-local addresses will be rejected. Save the form to apply this change.' '已恢复阻止 Webhook 指向内网/回环地址：指向回环、内网或链路本地地址的完成通知 Webhook 将再次被拒绝。保存表单后生效。'
assert_po_translation 'Renamed, but the leftover auto firewall rule (firewall.qnatter_%s) could not be removed automatically; remove it manually if the old port must not stay open.' '已重命名，但遗留的自动防火墙规则（firewall.qnatter_%s）未能自动删除；若旧端口不应继续开放，请手动删除该规则。'

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

	printf 'stale status\n' > "$tmp/stale-status.json"
	QNATTER_STATUS_FILE="$tmp/stale-status.json" QNATTER_GO_BIN="$tmp/qnatter-go-bin" \
		"$ROOT/qnatter/files/qnatter-run" "$tmp/stale-runtime.log" delta
	grep -qx 'go:delta' "$tmp/stale-runtime.log" || fail "qnatter-run with status file must execute qnatter-go"
	[ ! -e "$tmp/stale-status.json" ] || fail "qnatter-run must clear stale status before starting qnatter-go"

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
