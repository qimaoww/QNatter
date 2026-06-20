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

natter_runtime_slug() {
	printf '%s' "${1:-default}" | awk '
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

natter_json_escape() {
	printf '%s' "$1" | awk 'BEGIN { ORS = ""; tab = sprintf("%c", 9); cr = sprintf("%c", 13) } { if (NR > 1) printf "\\n"; gsub(/\\/, "\\\\"); gsub(/"/, "\\\""); gsub(cr, "\\r"); gsub(tab, "\\t"); printf "%s", $0 }'
}

natter_status_port() {
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
		"$(natter_status_port "$inner_port")" \
		"$(natter_json_escape "$outer_ip")" \
		"$(natter_status_port "$outer_port")" \
		"$(natter_json_escape "$now")" \
		"$(natter_json_escape "$message")" > "$file"
}
