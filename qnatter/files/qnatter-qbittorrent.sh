#!/bin/sh

# 边界告警只进 syslog：调用方常把本文件里的函数放在 $( ) 里并用 stderr 是否
# 为空当作"有没有出错"的判据（tests/test_qnatter_package.sh 就是这么断言 20 的），
# 所以绝不能在拒绝路径上写 stderr。qnatter-common.sh 已被调用方 source 时复用
# qnatter_warn，未 source（例如单独引用本文件）时退回直接调 logger。
qnatter_qb_warn() {
	local logger_bin="${QNATTER_LOGGER_BIN:-logger}"

	if command -v qnatter_warn >/dev/null 2>&1; then
		qnatter_warn "$*"
		return 0
	fi
	if command -v "$logger_bin" >/dev/null 2>&1; then
		"$logger_bin" -t qnatter "$*" >/dev/null 2>&1
	fi
	return 0
}

# 端口边界校验：只接受十进制 1..65535（0/0x/空 表示"未配置"）。
# 字符集与长度判定必须排在所有数值比较之前：dash 的 test 对超范围整数会打印
# "Illegal number"、busybox ash 的 test 会打印 "out of range" 并返回 2（两者都
# 写 stderr），busybox awk 与 gawk 对超大数的算术结果也不一致。先用长度把范围外
# 的输入挡掉，三条 shell（dash / busybox ash / busybox awk 环境）行为才一致。
qnatter_qb_valid_port() {
	local port="${1:-}"

	case "$port" in
		''|*[!0-9]*)
			qnatter_qb_warn "qnatter: rejecting non-numeric port value '${port}'"
			return 1
			;;
	esac

	[ "${#port}" -le 5 ] || {
		qnatter_qb_warn "qnatter: rejecting out-of-range port value '${port}'"
		return 1
	}
	# 0/00/000 是"端口未配置"的常规输入，静默拒绝（不产生告警噪音）。
	[ "$port" -eq 0 ] && return 1
	if [ "$port" -le 65535 ]; then
		return 0
	fi

	qnatter_qb_warn "qnatter: rejecting out-of-range port value '${port}'"
	return 1
}

qnatter_qb_select_listen_port() {
	local inner_port="$1"
	local outer_port="$2"
	local configured_port="${3:-0}"

	if qnatter_qb_valid_port "$configured_port"; then
		printf '%s\n' "$configured_port"
	elif qnatter_qb_valid_port "$outer_port"; then
		printf '%s\n' "$outer_port"
	elif qnatter_qb_valid_port "$inner_port"; then
		printf '%s\n' "$inner_port"
	else
		printf '%s\n' "0"
	fi
}

qnatter_qb_preferences_json() {
	local listen_port="$1"

	qnatter_qb_valid_port "$listen_port" || listen_port=0
	printf '{"listen_port":%s}' "$listen_port"
}

qnatter_qb_normalize_url() {
	local url="${1:-}"
	while [ "${url%/}" != "$url" ]; do
		url="${url%/}"
	done
	printf '%s\n' "$url"
}

qnatter_qb_write_notify_env() {
	local path="$1"
	local tmp="${path}.$$"
	local old_umask
	shift
	[ $(( $# % 2 )) -eq 0 ] || return 1
	old_umask="$(umask)"
	umask 077
	{
		while [ "$#" -gt 0 ]; do
			printf '%s=%s\n' "$1" "$(printf '%s' "$2" | sed "s/'/'\\\\''/g; s/^/'/; s/$/'/")"
			shift 2
		done
	} > "$tmp" || {
		umask "$old_umask"
		rm -f "$tmp"
		return 1
	}
	chmod 0600 "$tmp" || {
		umask "$old_umask"
		rm -f "$tmp"
		return 1
	}
	umask "$old_umask"
	mv "$tmp" "$path" || {
		rm -f "$tmp"
		return 1
	}
}
