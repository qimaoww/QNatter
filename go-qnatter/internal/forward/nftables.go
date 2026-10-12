package forward

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// 关于 mark 链的 hook priority：这里用 -149 而不是 -150。fw4 的 mangle_prerouting
// 与 OpenClash 注入到 inet fw4 的链都在 -150，而同一个 hook 点上、优先级相同（且
// 跨 family）时的求值顺序没有任何保证（实测按注册先后，整表重建后会翻转）。两边
// 都会 meta mark set，后执行者会覆盖前者的 mark。固定在 -149 让 QNatter 始终在
// fw4 之后求值，顺序确定。
func NftablesInitialRules() string {
	return `
table ip qnatter {
    chain qnatter_dnat { }
    chain qnatter_snat { }
    chain qnatter_mark { }
    chain prerouting {
        type nat hook prerouting priority -105; policy accept;
        jump qnatter_dnat;
    }
    chain mark_prerouting {
        type filter hook prerouting priority -149; policy accept;
        jump qnatter_mark;
    }
    chain output {
        type nat hook output priority -105; policy accept;
        jump qnatter_dnat;
    }
    chain postrouting {
        type nat hook postrouting priority 95; policy accept;
        jump qnatter_snat;
    }
    chain input {
        type nat hook input priority 95; policy accept;
        jump qnatter_snat;
    }
}
`
}

func NftablesRouteMarkInitialRules() string {
	return `
add chain ip qnatter qnatter_mark
add chain ip qnatter mark_prerouting { type filter hook prerouting priority -149; policy accept; }
add rule ip qnatter mark_prerouting jump qnatter_mark
`
}

// 规则拆成 spec（匹配+动作）与前缀两部分：spec 也要能用来在链里回找句柄
// （G-08：插入成功但句柄解析失败时按 spec 找回 handle 再删除）。
func NftablesDNATSpec(options StartOptions) string {
	return fmt.Sprintf(
		"ip daddr %s %s dport %d dnat to %s:%d",
		options.IP, protocolName(options.UDP), options.Port, options.TargetIP, options.TargetPort,
	)
}

func NftablesDNATRule(options StartOptions) string {
	return "insert rule ip qnatter qnatter_dnat " + NftablesDNATSpec(options)
}

func NftablesSNATSpec(options StartOptions) string {
	snatIP := options.SNATIP
	if snatIP == "" {
		snatIP = options.IP
	}
	return fmt.Sprintf(
		"ip daddr %s %s dport %d snat to %s",
		options.TargetIP, protocolName(options.UDP), options.TargetPort, snatIP,
	)
}

func NftablesSNATRule(options StartOptions) string {
	return "insert rule ip qnatter qnatter_snat " + NftablesSNATSpec(options)
}

func NftablesConnMarkSpec(options StartOptions, mark string) string {
	return fmt.Sprintf(
		"ip daddr %s %s dport %d ct mark set %s",
		options.IP, protocolName(options.UDP), options.Port, mark,
	)
}

func NftablesConnMarkRule(options StartOptions, mark string) string {
	return "insert rule ip qnatter qnatter_mark " + NftablesConnMarkSpec(options, mark)
}

func NftablesRouteMarkSpec(options StartOptions, mark string) string {
	return fmt.Sprintf(
		"ip saddr %s %s sport %d ct mark %s meta mark set %s",
		options.TargetIP, protocolName(options.UDP), options.TargetPort, mark, mark,
	)
}

func NftablesRouteMarkRule(options StartOptions, mark string) string {
	return "insert rule ip qnatter qnatter_mark " + NftablesRouteMarkSpec(options, mark)
}

func protocolName(udp bool) string {
	if udp {
		return "udp"
	}
	return "tcp"
}

// nftablesHandlePattern 匹配 `nft --handle` 输出中的规则句柄。
var nftablesHandlePattern = regexp.MustCompile(`# handle ([0-9]+)`)

// LookupRuleHandle 在 `nft list chain ...` 的输出里按规则文本找回句柄。
//
// nft 只支持按 handle 删除规则（`delete rule <chain> <spec>` 会报 syntax
// error），因此当 `nft --echo --handle insert` 没有回显句柄时，这是刚刚
// 插入的那条规则唯一的定位方式。
func LookupRuleHandle(listOutput string, spec string) (int, error) {
	if spec == "" {
		return 0, fmt.Errorf("unknown nftables rule spec")
	}
	for _, line := range strings.Split(listOutput, "\n") {
		if !strings.Contains(line, spec) {
			continue
		}
		match := nftablesHandlePattern.FindStringSubmatch(line)
		if len(match) != 2 {
			continue
		}
		handle, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		return handle, nil
	}
	return 0, fmt.Errorf("nftables rule handle not found for %q", spec)
}

func ParseNftablesHandle(output string) (int, error) {
	match := nftablesHandlePattern.FindStringSubmatch(output)
	if len(match) != 2 {
		return 0, fmt.Errorf("unknown nftables handle")
	}
	handle, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, err
	}
	return handle, nil
}
