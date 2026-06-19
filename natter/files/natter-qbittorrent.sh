#!/bin/sh

natter_qb_valid_port() {
	local port="${1:-}"

	case "$port" in
		''|*[!0-9]*) return 1 ;;
	esac

	[ "$port" -ge 1 ] && [ "$port" -le 65535 ]
}

natter_qb_select_listen_port() {
	local inner_port="$1"
	local outer_port="$2"
	local configured_port="${3:-0}"

	if natter_qb_valid_port "$configured_port"; then
		printf '%s\n' "$configured_port"
	elif natter_qb_valid_port "$outer_port"; then
		printf '%s\n' "$outer_port"
	elif natter_qb_valid_port "$inner_port"; then
		printf '%s\n' "$inner_port"
	else
		printf '%s\n' "0"
	fi
}

natter_qb_preferences_json() {
	local listen_port="$1"

	natter_qb_valid_port "$listen_port" || listen_port=0
	printf '{"listen_port":%s}' "$listen_port"
}

natter_qb_normalize_url() {
	local url="${1:-}"
	url="${url%/}"
	printf '%s\n' "$url"
}

natter_qb_write_notify_env() {
	local path="$1"
	shift
	umask 077
	{
		while [ "$#" -gt 0 ]; do
			printf '%s=%s\n' "$1" "$(printf '%s' "$2" | sed "s/'/'\\\\''/g; s/^/'/; s/$/'/")"
			shift 2
		done
	} > "$path"
}
