#!/bin/sh

qnatter_qb_valid_port() {
	local port="${1:-}"

	case "$port" in
		''|*[!0-9]*) return 1 ;;
	esac

	[ "${#port}" -le 5 ] || return 1
	[ "$port" -ge 1 ] && [ "$port" -le 65535 ]
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
