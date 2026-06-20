#!/bin/sh

NATTER_ARGS=""

natter_bool() {
	local value="${1:-0}"
	case "$value" in
		1|on|true|yes|enabled) return 0 ;;
	esac
	return 1
}

natter_add_arg() {
	NATTER_ARGS="${NATTER_ARGS}
$1"
}

natter_add_arg_value() {
	[ -n "${2:-}" ] || return 0
	natter_add_arg "$1"
	natter_add_arg "$2"
}

natter_add_list_values() {
	local opt="$1"
	shift
	local item
	for item in "$@"; do
		[ -n "$item" ] || continue
		natter_add_arg "$opt"
		natter_add_arg "$item"
	done
}

natter_forward_method_or_auto() {
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

natter_build_args() {
	local forward_method

	NATTER_ARGS=""
	forward_method="$(natter_forward_method_or_auto "${NATTER_FORWARD_METHOD:-auto}")"

	[ "${NATTER_PROTOCOL:-tcp}" = "udp" ] && natter_add_arg "-u"
	natter_bool "${NATTER_VERBOSE:-0}" && natter_add_arg "-v"
	natter_bool "${NATTER_EXIT_WHEN_CHANGED:-0}" && natter_add_arg "-q"
	natter_bool "${NATTER_UPNP:-0}" && natter_add_arg "-U"

	natter_add_arg_value "-k" "${NATTER_KEEPALIVE_INTERVAL:-15}"

	# shellcheck disable=SC2086
	natter_add_list_values "-s" ${NATTER_STUN_SERVERS:-}
	natter_add_arg_value "-h" "${NATTER_KEEPALIVE_SERVER:-}"
	natter_add_arg_value "-i" "${NATTER_BIND_VALUE:-}"
	natter_add_arg_value "-b" "${NATTER_BIND_PORT:-}"
	[ "$forward_method" != "auto" ] && natter_add_arg_value "-m" "$forward_method"
	natter_add_arg_value "-t" "${NATTER_TARGET_IP:-}"
	natter_add_arg_value "-p" "${NATTER_TARGET_PORT:-}"
	natter_bool "${NATTER_RETRY_TARGET:-0}" && natter_add_arg "-r"
	natter_add_arg_value "-e" "${NATTER_NOTIFY_PATH:-}"
}

natter_slug() {
	printf '%s' "${1:-default}" | tr -c 'A-Za-z0-9_' '_'
}

natter_json_escape() {
	printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

natter_write_status_json() {
	local file="$1"
	local instance="$2"
	local protocol="$3"
	local inner_ip="$4"
	local inner_port="$5"
	local outer_ip="$6"
	local outer_port="$7"
	local message="$8"
	local now

	now="$(date '+%Y-%m-%d %H:%M:%S')"
	mkdir -p "$(dirname "$file")"
	printf '{"instance":"%s","protocol":"%s","inner_ip":"%s","inner_port":%s,"outer_ip":"%s","outer_port":%s,"updated_at":"%s","message":"%s"}\n' \
		"$(natter_json_escape "$instance")" \
		"$(natter_json_escape "$protocol")" \
		"$(natter_json_escape "$inner_ip")" \
		"${inner_port:-0}" \
		"$(natter_json_escape "$outer_ip")" \
		"${outer_port:-0}" \
		"$(natter_json_escape "$now")" \
		"$(natter_json_escape "$message")" > "$file"
}
