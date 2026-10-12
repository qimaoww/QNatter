package forward

import (
	"fmt"
	"net/netip"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// 本文件实现"同一监听地址是否已经被其它监听者占用"的探测（G-12）。
//
// 背景：转发监听套接字必须与 keep-alive socket 共用同一个内网 addr:port
// （keep-alive 负责保住 NAT 映射，监听套接字负责接流量），因此必须带上
// SO_REUSEPORT。副作用是同一 addr:port 上的第二个监听者不会收到
// EADDRINUSE，而是被内核放进同一个 REUSEPORT 组：新连接按四元组哈希落到
// 组内任意一个套接字。两个实例共用一个端口时，连接就会时而被转发到 A 的
// target、时而被转发到 B 的 target（"时好时坏"且毫无报错）。
//
// 因此绑定之前先读 /proc/net/{tcp,udp}：该地址/端口上若已有别的监听者，
// 直接以明确错误拒绝共享（错误信息会经 engine 的重试日志输出到用户），
// 而不是让 REUSEPORT 静默把连接分流到不同 target。
//
// 探针只认"同一地址"与"对方绑 0.0.0.0"两种占用：本程序绑 0.0.0.0 而对方
// 绑在更具体的地址上时，内核会把流量让给更具体的套接字，不属于随机分流。
const (
	procNetTCP = "/proc/net/tcp"
	procNetUDP = "/proc/net/udp"
	// procNetTCPListenState 是 /proc/net/tcp 中 LISTEN 状态的十六进制值
	// （TCP_LISTEN = 10）。
	procNetTCPListenState = "0A"
)

// sharedListenPortError 返回非 nil 表示 ip:port 已经被别的监听者占用
// （含第二个 QNatter 实例）。跳过探测的情况一律返回 nil：
// 非 Linux、端口为 0（由内核分配）、地址不是 IPv4 字面量。
func sharedListenPortError(ip string, port int, udp bool) error {
	if runtime.GOOS != "linux" || port <= 0 {
		return nil
	}
	want, ok := parseBindAddress(ip)
	if !ok {
		return nil
	}

	path := procNetTCP
	protocol := "tcp"
	if udp {
		path = procNetUDP
		protocol = "udp"
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		// 读不到 /proc 时无法判定，放行（保持原有行为），不因为探测失败
		// 而拒绝启动。
		return nil
	}
	if !hasProcNetListener(string(raw), want, uint16(port), udp) {
		return nil
	}
	return fmt.Errorf(
		"cannot listen on %s:%d: another process is already bound to this %s port; "+
			"a second QNatter instance sharing the port would split inbound connections between different forward targets. "+
			"Stop the other instance or move this one to another bind port (-b)",
		ip, port, protocol,
	)
}

func parseBindAddress(ip string) (netip.Addr, bool) {
	if ip == "" {
		return netip.IPv4Unspecified(), true
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is4() {
		return netip.Addr{}, false
	}
	return addr, true
}

// hasProcNetListener 在 /proc/net/tcp 或 /proc/net/udp 的内容里查找与
// want:port 冲突的监听者。
func hasProcNetListener(content string, want netip.Addr, port uint16, udp bool) bool {
	lines := strings.Split(content, "\n")
	// 第一行是表头（"sl local_address rem_address st ..."）。
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		localAddr, localPort, ok := parseProcNetEndpoint(fields[1])
		if !ok || localPort != port || !addrConflicts(want, localAddr) {
			continue
		}
		if udp {
			// 监听用 UDP 套接字未连接（remote 为 0）；已连接的套接字
			// （keep-alive/STUN）只是共用端口，不是分流来源。
			remotePort, ok := parseProcNetPort(fields[2])
			if !ok || remotePort != 0 {
				continue
			}
			return true
		}
		if !strings.EqualFold(fields[3], procNetTCPListenState) {
			continue
		}
		return true
	}
	return false
}

// addrConflicts 判断占用地址 entry 是否与本程序要绑定的 want 冲突。
func addrConflicts(want, entry netip.Addr) bool {
	if entry.IsUnspecified() {
		return true
	}
	return want == entry
}

func parseProcNetEndpoint(field string) (netip.Addr, uint16, bool) {
	host, portText, ok := strings.Cut(field, ":")
	if !ok || len(host) != 8 {
		return netip.Addr{}, 0, false
	}
	raw, err := strconv.ParseUint(host, 16, 32)
	if err != nil {
		return netip.Addr{}, 0, false
	}
	port, err := strconv.ParseUint(portText, 16, 16)
	if err != nil {
		return netip.Addr{}, 0, false
	}
	// /proc/net/tcp 里的 IPv4 地址是 32 位小端序：0x0100007F 是 127.0.0.1。
	addr := netip.AddrFrom4([4]byte{byte(raw), byte(raw >> 8), byte(raw >> 16), byte(raw >> 24)})
	return addr, uint16(port), true
}

func parseProcNetPort(field string) (uint16, bool) {
	_, port, ok := parseProcNetEndpoint(field)
	return port, ok
}
