#!/bin/sh

QNATTER_ARGS=""

qnatter_bool() {
	local value="${1:-0}"
	case "$value" in
		1|on|true|yes|enabled) return 0 ;;
	esac
	return 1
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

qnatter_build_args() {
	local forward_method

	QNATTER_ARGS=""
	forward_method="$(qnatter_forward_method_or_auto "${QNATTER_FORWARD_METHOD:-auto}")"

	[ "${QNATTER_PROTOCOL:-tcp}" = "udp" ] && qnatter_add_arg "-u"
	qnatter_bool "${QNATTER_VERBOSE:-0}" && qnatter_add_arg "-v"
	qnatter_bool "${QNATTER_EXIT_WHEN_CHANGED:-0}" && qnatter_add_arg "-q"
	qnatter_bool "${QNATTER_UPNP:-0}" && qnatter_add_arg "-U"

	qnatter_add_arg_value "-k" "${QNATTER_KEEPALIVE_INTERVAL:-15}"

	# shellcheck disable=SC2086
	qnatter_add_list_values "-s" ${QNATTER_STUN_SERVERS:-}
	qnatter_add_arg_value "-h" "${QNATTER_KEEPALIVE_SERVER:-}"
	qnatter_add_arg_value "-i" "${QNATTER_BIND_VALUE:-}"
	qnatter_add_arg_value "-b" "${QNATTER_BIND_PORT:-}"
	[ "$forward_method" != "auto" ] && qnatter_add_arg_value "-m" "$forward_method"
	qnatter_add_arg_value "-t" "${QNATTER_TARGET_IP:-}"
	qnatter_add_arg_value "-p" "${QNATTER_TARGET_PORT:-}"
	qnatter_bool "${QNATTER_RETRY_TARGET:-0}" && qnatter_add_arg "-r"
	qnatter_add_arg_value "-e" "${QNATTER_NOTIFY_PATH:-}"
}

qnatter_slug() {
	printf '%s' "${1:-default}" | tr -c 'A-Za-z0-9_' '_'
}

qnatter_runtime_slug() {
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
