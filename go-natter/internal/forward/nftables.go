package forward

import (
	"fmt"
	"regexp"
	"strconv"
)

func NftablesInitialRules() string {
	return `
table ip natter {
    chain natter_dnat { }
    chain natter_snat { }
    chain natter_mark { }
    chain prerouting {
        type nat hook prerouting priority -105; policy accept;
        jump natter_dnat;
    }
    chain mark_prerouting {
        type filter hook prerouting priority -150; policy accept;
        jump natter_mark;
    }
    chain output {
        type nat hook output priority -105; policy accept;
        jump natter_dnat;
    }
    chain postrouting {
        type nat hook postrouting priority 95; policy accept;
        jump natter_snat;
    }
    chain input {
        type nat hook input priority 95; policy accept;
        jump natter_snat;
    }
}
`
}

func NftablesRouteMarkInitialRules() string {
	return `
add chain ip natter natter_mark
add chain ip natter mark_prerouting { type filter hook prerouting priority -150; policy accept; }
add rule ip natter mark_prerouting jump natter_mark
`
}

func NftablesDNATRule(options StartOptions) string {
	proto := "tcp"
	if options.UDP {
		proto = "udp"
	}
	return fmt.Sprintf(
		"insert rule ip natter natter_dnat ip daddr %s %s dport %d dnat to %s:%d",
		options.IP, proto, options.Port, options.TargetIP, options.TargetPort,
	)
}

func NftablesSNATRule(options StartOptions) string {
	proto := "tcp"
	if options.UDP {
		proto = "udp"
	}
	snatIP := options.SNATIP
	if snatIP == "" {
		snatIP = options.IP
	}
	return fmt.Sprintf(
		"insert rule ip natter natter_snat ip daddr %s %s dport %d snat to %s",
		options.TargetIP, proto, options.TargetPort, snatIP,
	)
}

func NftablesConnMarkRule(options StartOptions, mark string) string {
	proto := "tcp"
	if options.UDP {
		proto = "udp"
	}
	return fmt.Sprintf(
		"insert rule ip natter natter_mark ip daddr %s %s dport %d ct mark set %s",
		options.IP, proto, options.Port, mark,
	)
}

func NftablesRouteMarkRule(options StartOptions, mark string) string {
	proto := "tcp"
	if options.UDP {
		proto = "udp"
	}
	return fmt.Sprintf(
		"insert rule ip natter natter_mark ip saddr %s %s sport %d ct mark %s meta mark set %s",
		options.TargetIP, proto, options.TargetPort, mark, mark,
	)
}

func ParseNftablesHandle(output string) (int, error) {
	match := regexp.MustCompile(`# handle ([0-9]+)`).FindStringSubmatch(output)
	if len(match) != 2 {
		return 0, fmt.Errorf("unknown nftables handle")
	}
	handle, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, err
	}
	return handle, nil
}
