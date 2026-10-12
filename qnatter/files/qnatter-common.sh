#!/bin/sh

QNATTER_ARGS=""

qnatter_bool() {
	local value="${1:-0}"
	case "$value" in
		1|on|true|yes|enabled) return 0 ;;
	esac
	return 1
}

qnatter_warn() {
	local logger_bin="${QNATTER_LOGGER_BIN:-logger}"

	command -v "$logger_bin" >/dev/null 2>&1 || return 0
	"$logger_bin" -t qnatter "$*" >/dev/null 2>&1 || true
}

# UCI 允许 section/option 名包含 '-'、'.' 等字符，而 /lib/functions.sh 的
# config_get 只接受 [A-Za-z0-9_]：其余情况静默不赋值，实例会被当成 disabled。
# 这里先调用框架函数（保持与框架及测试替身一致的语义），取不到值时再直读
# config_load 导出的 CONFIG_<sanitized>_ 变量作为兼容路径。
qnatter_config_get() {
	local __var="$1"
	local section="$2"
	local option="$3"
	local default="${4:-}"
	local __qnatter_sentinel="__qnatter_unset__"
	local __qnatter_value="$__qnatter_sentinel"
	local __qnatter_sanitized

	# 内部局部变量统一加 __qnatter_ 前缀：框架（以及测试替身）的 config_get
	# 也用 eval 给调用方指定的变量赋值，同名局部变量会互相遮蔽。
	# 用哨兵值判断框架是否真的完成了赋值（section 名非法时它会静默不赋值）。
	config_get __qnatter_value "$section" "$option" "$__qnatter_sentinel" 2>/dev/null
	if [ "$__qnatter_value" = "$__qnatter_sentinel" ]; then
		__qnatter_sanitized="$(printf '%s' "$section" | tr -c 'A-Za-z0-9_' '_')"
		eval "__qnatter_value=\"\${CONFIG_${__qnatter_sanitized}_${option}-$__qnatter_sentinel}\""
		if [ "$__qnatter_value" = "$__qnatter_sentinel" ] || [ -z "$__qnatter_value" ]; then
			__qnatter_value="$default"
		fi
	fi

	eval "$__var=\$__qnatter_value"
}

qnatter_config_get_bool() {
	local __var="$1"
	local section="$2"
	local option="$3"
	local default="${4:-0}"
	local __qnatter_raw

	qnatter_config_get __qnatter_raw "$section" "$option" "$default"
	case "$__qnatter_raw" in
		1|on|true|yes|enabled) __qnatter_raw=1 ;;
		*) __qnatter_raw=0 ;;
	esac
	eval "$__var=\$__qnatter_raw"
}

# 关闭/恢复 glob 展开。展开 UCI 列表值时必须先关闭 glob，否则值里的
# '*'/'?' 会被展开成本地文件名并进入 qnatter-go 的参数列表。
qnatter_glob_disable() {
	case $- in
		*f*) QNATTER_GLOB_RESTORE=0 ;;
		*) set -f; QNATTER_GLOB_RESTORE=1 ;;
	esac
}

qnatter_glob_restore() {
	[ "${QNATTER_GLOB_RESTORE:-0}" = "1" ] && set +f
	QNATTER_GLOB_RESTORE=0
	return 0
}

# 按空白切分一个列表字符串，逐项交给回调（切分期间关闭 glob）。
qnatter_split_list() {
	local callback="$1"
	local values="${2:-}"
	local item

	qnatter_glob_disable
	for item in $values; do
		[ -n "$item" ] || continue
		"$callback" "$item"
	done
	qnatter_glob_restore
	return 0
}

# STUN 值只允许域名 / IPv4 / IPv6 字面量加可选端口；'*'、'?'、空白、引号、
# '$'、反引号以及以 '-' 开头的值（会被 qnatter-go 当成选项）一律拒绝。
qnatter_stun_value_allowed() {
	local value="${1:-}"

	[ -n "$value" ] || return 1
	case "$value" in
		-*) return 1 ;;
	esac
	value="$(printf '%s' "$value" | tr -d '[]')"
	case "$value" in
		*[!A-Za-z0-9._:-]*) return 1 ;;
	esac
	return 0
}

# 传给 curl 的 URL 必须是 http(s)，且不能被 curl 当成选项（P1-5）。
qnatter_url_allowed() {
	local url="${1:-}"

	case "$url" in
		-*) return 1 ;;
	esac
	case "$url" in
		http://?*|https://?*) ;;
		*) return 1 ;;
	esac
	case "$url" in
		*[[:space:][:cntrl:]]*) return 1 ;;
	esac
	return 0
}

# 日志里剥离 URL 中的 userinfo（http://user:pass@host -> http://***@host）。
qnatter_sanitize_url() {
	printf '%s' "${1:-}" | sed -e 's#\(://\)[^/@[:space:]]*@#\1***@#'
}

# 创建 0700 的私有临时目录（默认落在 QNatter 的 run 目录下），用于存放
# cookie / 凭据等敏感文件；失败时回落到 /tmp（不回落到 chmod /tmp）。
qnatter_private_tmpdir() {
	local base="${1:-/var/run/qnatter}"
	local dir

	mkdir -p "$base" 2>/dev/null || base="/tmp"
	if [ "$base" != "/tmp" ]; then
		chmod 0700 "$base" 2>/dev/null || true
	fi

	dir="$(mktemp -d "${base}/.qnatter-XXXXXX" 2>/dev/null)" || dir=""
	if [ -z "$dir" ]; then
		dir="${base}/.qnatter-$$"
		(umask 077; mkdir "$dir" 2>/dev/null) || return 1
	fi
	chmod 0700 "$dir" 2>/dev/null || true
	printf '%s\n' "$dir"
}

qnatter_add_arg() {
	QNATTER_ARGS="${QNATTER_ARGS}
$1"
}

qnatter_add_arg_value() {
	[ -n "${2:-}" ] || return 0
	qnatter_add_arg "$1"
	qnatter_add_arg "$2"
}

qnatter_add_list_values() {
	local opt="$1"
	shift
	local item
	for item in "$@"; do
		[ -n "$item" ] || continue
		qnatter_add_arg "$opt"
		qnatter_add_arg "$item"
	done
}

qnatter_forward_method_or_auto() {
	local method="${1:-auto}"

	case "$method" in
		auto|none|test|socket|nftables|nftables-snat)
			printf '%s\n' "$method"
			;;
		socat|gost)
			if command -v "$method" >/dev/null 2>&1; then
				printf '%s\n' "$method"
			else
				printf '%s\n' "auto"
			fi
			;;
		*)
			printf '%s\n' "auto"
			;;
	esac
}

qnatter_add_stun_server_arg() {
	# 进入 qnatter-go 参数列表前再过一次字符集白名单：切分只保证不做 glob
	# 展开，'*' 之类的值仍会被 go 当成参数。
	if ! qnatter_stun_value_allowed "$1"; then
		qnatter_warn "qnatter: ignoring unsupported STUN server value"
		return 0
	fi
	qnatter_add_arg "-s"
	qnatter_add_arg "$1"
}

qnatter_build_args() {
	local forward_method

	QNATTER_ARGS=""
	forward_method="$(qnatter_forward_method_or_auto "${QNATTER_FORWARD_METHOD:-auto}")"

	[ "${QNATTER_PROTOCOL:-tcp}" = "udp" ] && qnatter_add_arg "-u"
	qnatter_bool "${QNATTER_VERBOSE:-0}" && qnatter_add_arg "-v"
	qnatter_bool "${QNATTER_EXIT_WHEN_CHANGED:-0}" && qnatter_add_arg "-q"
	qnatter_bool "${QNATTER_UPNP:-0}" && qnatter_add_arg "-U"

	qnatter_add_arg_value "-k" "${QNATTER_KEEPALIVE_INTERVAL:-15}"

	# STUN 列表逐项切分（切分期间关闭 glob），避免 UCI 值中的 '*'/'?' 被
	# 展开成本地文件名后进入 qnatter-go 的参数列表。
	qnatter_split_list qnatter_add_stun_server_arg "${QNATTER_STUN_SERVERS:-}"
	qnatter_add_arg_value "-h" "${QNATTER_KEEPALIVE_SERVER:-}"
	qnatter_add_arg_value "-i" "${QNATTER_BIND_VALUE:-}"
	qnatter_add_arg_value "-b" "${QNATTER_BIND_PORT:-}"
	[ "$forward_method" != "auto" ] && qnatter_add_arg_value "-m" "$forward_method"
	qnatter_add_arg_value "-t" "${QNATTER_TARGET_IP:-}"
	qnatter_add_arg_value "-p" "${QNATTER_TARGET_PORT:-}"
	qnatter_bool "${QNATTER_RETRY_TARGET:-0}" && qnatter_add_arg "-r"
	qnatter_add_arg_value "-e" "${QNATTER_NOTIFY_PATH:-}"
}

# 文件/日志名一律走这里（_xNN 转义，避免不同 section 名折叠到同一文件）。
# 早期还有过一个把 '-'、'.' 直接折叠成 '_' 的 qnatter_slug()，运行时代码零调用，
# 已在第三轮删除；只保留不会产生文件名冲突的实现。
qnatter_runtime_slug() {
	# LC_ALL=C 固定为按字节处理：gawk 在 UTF-8 locale 下会对多字节字符按字符
	# 计数，busybox awk 按字节计数，两者会为同一个 section 生成不同文件名。
	printf '%s' "${1:-default}" | LC_ALL=C awk '
		BEGIN {
			for (i = 1; i < 128; i++)
				ord[sprintf("%c", i)] = i
		}
		{
			for (i = 1; i <= length($0); i++) {
				char = substr($0, i, 1)
				next_three = substr($0, i + 1, 3)
				if (char == "_" && next_three ~ /^x[0-9A-Fa-f][0-9A-Fa-f]$/)
					out = out "_x5f"
				else if (char ~ /^[A-Za-z0-9_]$/)
					out = out char
				else if (char in ord)
					out = out sprintf("_x%02x", ord[char])
				else
					out = out "_"
			}
		}
		END {
			if (out == "")
				out = "default"
			print out
		}
	'
}

qnatter_json_escape() {
	printf '%s' "$1" | awk 'BEGIN { ORS = ""; tab = sprintf("%c", 9); cr = sprintf("%c", 13) } { if (NR > 1) printf "\\n"; gsub(/\\/, "\\\\"); gsub(/"/, "\\\""); gsub(cr, "\\r"); gsub(tab, "\\t"); printf "%s", $0 }'
}

qnatter_status_port() {
	local port="${1:-}"

	case "$port" in
		''|*[!0-9]*) printf '%s\n' "0"; return 0 ;;
	esac
	[ "${#port}" -le 5 ] || {
		printf '%s\n' "0"
		return 0
	}
	awk -v port="$port" 'BEGIN { port += 0; if (port >= 0 && port <= 65535) printf "%d\n", port; else printf "0\n" }'
}

qnatter_write_status_json() {
	local file="$1"
	local instance="$2"
	local protocol="$3"
	local inner_ip="$4"
	local inner_port="$5"
	local outer_ip="$6"
	local outer_port="$7"
	local message="$8"
	local now
	local tmp

	now="$(date '+%Y-%m-%d %H:%M:%S')"
	mkdir -p "$(dirname "$file")"
	tmp="${file}.$$"
	printf '{"instance":"%s","protocol":"%s","inner_ip":"%s","inner_port":%s,"outer_ip":"%s","outer_port":%s,"updated_at":"%s","message":"%s"}\n' \
		"$(qnatter_json_escape "$instance")" \
		"$(qnatter_json_escape "$protocol")" \
		"$(qnatter_json_escape "$inner_ip")" \
		"$(qnatter_status_port "$inner_port")" \
		"$(qnatter_json_escape "$outer_ip")" \
		"$(qnatter_status_port "$outer_port")" \
		"$(qnatter_json_escape "$now")" \
		"$(qnatter_json_escape "$message")" > "$tmp" || {
		rm -f "$tmp"
		return 1
	}
	mv "$tmp" "$file" || {
		rm -f "$tmp"
		return 1
	}
}
