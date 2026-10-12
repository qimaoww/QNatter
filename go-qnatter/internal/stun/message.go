package stun

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"net/netip"
)

const (
	bindingRequest       uint16 = 0x0001
	bindingSuccess       uint16 = 0x0101
	attrMappedAddress    uint16 = 0x0001
	attrXORMappedAddress uint16 = 0x0020
	// attrFingerprint 是 RFC 5389 §15.5 的 FINGERPRINT 属性。
	attrFingerprint uint16 = 0x8028
	// fingerprintXOR 是 FINGERPRINT 的计算常量：CRC32 ^ 0x5354554e。
	fingerprintXOR uint32 = 0x5354554e
	// addrFamilyIPv4 / addrFamilyIPv6 是地址属性的 family 字段取值
	// （RFC 5389 §15.1）。
	addrFamilyIPv4 byte   = 0x01
	addrFamilyIPv6 byte   = 0x02
	MagicCookie    uint32 = 0x2112a442
)

func BuildBindingRequest(txid [12]byte) []byte {
	msg := make([]byte, 20)
	binary.BigEndian.PutUint16(msg[0:2], bindingRequest)
	binary.BigEndian.PutUint16(msg[2:4], 0)
	binary.BigEndian.PutUint32(msg[4:8], MagicCookie)
	copy(msg[8:20], txid[:])
	return msg
}

// ParseMappedAddress 解析 STUN binding success 响应里的映射地址。
//
// 校验：报文类型高 2 位必须为 0（RFC 5389 §6）、message type、magic cookie、
// transaction id、属性长度，以及存在时的 FINGERPRINT（§15.5，CRC32 必须
// 匹配，否则报文可能被改写/损坏）。
//
// MESSAGE-INTEGRITY（0x0008）需要共享密钥，而本程序使用无认证的 STUN
// （RFC 5389 §10.1.2 的 short-term/long-term 凭据都不存在），因此无法校验：
// 这里的取舍是"不因为存在 MESSAGE-INTEGRITY 就拒绝报文，也不把响应视为
// 已认证"，只依赖 magic cookie + transaction id + FINGERPRINT 做完整性判断。
//
// 未知属性、被忽略的属性继续跳过，不因此拒绝报文（互通要求：真实服务器
// 会带回 SOFTWARE、RESPONSE-ORIGIN、ALTERNATE-SERVER 等属性）。
func ParseMappedAddress(msg []byte, txid [12]byte) (netip.AddrPort, error) {
	if len(msg) < 20 {
		return netip.AddrPort{}, errors.New("short STUN response")
	}
	// RFC 5389 §6：每条 STUN 报文的最高 2 位必须为 0，其余取值是其它
	// 协议（如 RFC 7983 多路复用标记）的报文，不能被当成 STUN 解析。
	if msg[0]&0xc0 != 0 {
		return netip.AddrPort{}, errors.New("invalid STUN message type: top two bits must be zero")
	}
	if binary.BigEndian.Uint16(msg[0:2]) != bindingSuccess {
		return netip.AddrPort{}, errors.New("not a STUN binding success response")
	}
	if binary.BigEndian.Uint32(msg[4:8]) != MagicCookie {
		return netip.AddrPort{}, errors.New("invalid STUN magic cookie")
	}
	if string(msg[8:20]) != string(txid[:]) {
		return netip.AddrPort{}, errors.New("STUN transaction id mismatch")
	}

	size := int(binary.BigEndian.Uint16(msg[2:4]))
	if len(msg) < 20+size {
		return netip.AddrPort{}, errors.New("truncated STUN attributes")
	}

	// XOR-MAPPED-ADDRESS 的 XOR 掩码是 (magic cookie || transaction id)：
	// IPv4 只用前 4 字节，IPv6 用满 16 字节（RFC 5389 §15.2）。
	var xorMask [16]byte
	binary.BigEndian.PutUint32(xorMask[0:4], MagicCookie)
	copy(xorMask[4:], txid[:])

	payload := msg[20 : 20+size]
	offset := 0
	var address netip.AddrPort
	found := false
	for len(payload) >= 4 {
		typ := binary.BigEndian.Uint16(payload[0:2])
		attrLen := int(binary.BigEndian.Uint16(payload[2:4]))
		if len(payload) < 4+attrLen {
			return netip.AddrPort{}, errors.New("truncated STUN attribute")
		}

		value := payload[4 : 4+attrLen]
		switch {
		case !found && (typ == attrMappedAddress || typ == attrXORMappedAddress):
			// 只取第一个映射地址属性；循环必须继续跑完，否则排在地址
			// 属性之后的 FINGERPRINT（RFC 5389 §15.5 要求它放最后）
			// 永远不会被校验。
			parsed, err := parseAddressAttribute(typ, value, xorMask)
			if err != nil {
				return netip.AddrPort{}, err
			}
			address = parsed
			found = true
		case typ == attrFingerprint:
			if err := verifyFingerprint(msg[:20+offset], value); err != nil {
				return netip.AddrPort{}, err
			}
		}

		padded := (attrLen + 3) &^ 3
		if len(payload) < 4+padded {
			return netip.AddrPort{}, errors.New("truncated STUN attribute padding")
		}
		offset += 4 + padded
		payload = payload[4+padded:]
	}

	if !found {
		return netip.AddrPort{}, errors.New("mapped address attribute not found")
	}
	return address, nil
}

// verifyFingerprint 校验 FINGERPRINT 属性（RFC 5389 §15.5）：CRC32 覆盖从
// 报文起始到 FINGERPRINT 属性之前的全部字节（报文头的 length 字段本身已
// 包含 FINGERPRINT，因此直接对 head 求 CRC），再异或 0x5354554e。
func verifyFingerprint(head []byte, value []byte) error {
	if len(value) != 4 {
		return fmt.Errorf("invalid STUN FINGERPRINT length %d", len(value))
	}
	if len(head) < 20 {
		return errors.New("invalid STUN FINGERPRINT position")
	}
	want := binary.BigEndian.Uint32(value)
	got := crc32.ChecksumIEEE(head) ^ fingerprintXOR
	if want != got {
		return fmt.Errorf("invalid STUN FINGERPRINT: got %#08x want %#08x", want, got)
	}
	return nil
}

func parseAddressAttribute(typ uint16, value []byte, xorMask [16]byte) (netip.AddrPort, error) {
	if len(value) < 4 {
		return netip.AddrPort{}, errors.New("short STUN address attribute")
	}
	port := binary.BigEndian.Uint16(value[2:4])

	switch value[1] {
	case addrFamilyIPv4:
		if len(value) < 8 {
			return netip.AddrPort{}, errors.New("short STUN IPv4 address attribute")
		}
		ip := [4]byte{}
		copy(ip[:], value[4:8])
		if typ == attrXORMappedAddress {
			port ^= uint16(MagicCookie >> 16)
			for i := range ip {
				ip[i] ^= xorMask[i]
			}
		}
		return netip.AddrPortFrom(netip.AddrFrom4(ip), port), nil
	case addrFamilyIPv6:
		// RFC 5389 §15.2：IPv6 的地址字段是 16 字节，XOR-MAPPED-ADDRESS
		// 用完整的 16 字节掩码异或（以前只支持 family=1，合法的 IPv6
		// 映射被直接拒绝）。
		if len(value) < 20 {
			return netip.AddrPort{}, errors.New("short STUN IPv6 address attribute")
		}
		ip := [16]byte{}
		copy(ip[:], value[4:20])
		if typ == attrXORMappedAddress {
			port ^= uint16(MagicCookie >> 16)
			for i := range ip {
				ip[i] ^= xorMask[i]
			}
		}
		return netip.AddrPortFrom(netip.AddrFrom16(ip), port), nil
	default:
		return netip.AddrPort{}, fmt.Errorf("unsupported STUN address family %d", value[1])
	}
}
