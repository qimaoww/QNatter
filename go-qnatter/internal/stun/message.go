package stun

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
)

const (
	bindingRequest       uint16 = 0x0001
	bindingSuccess       uint16 = 0x0101
	attrMappedAddress    uint16 = 0x0001
	attrXORMappedAddress uint16 = 0x0020
	MagicCookie          uint32 = 0x2112a442
)

func BuildBindingRequest(txid [12]byte) []byte {
	msg := make([]byte, 20)
	binary.BigEndian.PutUint16(msg[0:2], bindingRequest)
	binary.BigEndian.PutUint16(msg[2:4], 0)
	binary.BigEndian.PutUint32(msg[4:8], MagicCookie)
	copy(msg[8:20], txid[:])
	return msg
}

func ParseMappedAddress(msg []byte, txid [12]byte) (netip.AddrPort, error) {
	if len(msg) < 20 {
		return netip.AddrPort{}, errors.New("short STUN response")
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

	payload := msg[20 : 20+size]
	for len(payload) >= 4 {
		typ := binary.BigEndian.Uint16(payload[0:2])
		attrLen := int(binary.BigEndian.Uint16(payload[2:4]))
		if len(payload) < 4+attrLen {
			return netip.AddrPort{}, errors.New("truncated STUN attribute")
		}

		value := payload[4 : 4+attrLen]
		if typ == attrMappedAddress || typ == attrXORMappedAddress {
			return parseAddressAttribute(typ, value)
		}

		padded := (attrLen + 3) &^ 3
		if len(payload) < 4+padded {
			return netip.AddrPort{}, errors.New("truncated STUN attribute padding")
		}
		payload = payload[4+padded:]
	}

	return netip.AddrPort{}, errors.New("mapped address attribute not found")
}

func parseAddressAttribute(typ uint16, value []byte) (netip.AddrPort, error) {
	if len(value) < 8 {
		return netip.AddrPort{}, errors.New("short STUN address attribute")
	}
	if value[1] != 1 {
		return netip.AddrPort{}, fmt.Errorf("unsupported STUN address family %d", value[1])
	}

	port := binary.BigEndian.Uint16(value[2:4])
	ip := [4]byte{}
	copy(ip[:], value[4:8])

	if typ == attrXORMappedAddress {
		port ^= uint16(MagicCookie >> 16)
		cookie := [4]byte{}
		binary.BigEndian.PutUint32(cookie[:], MagicCookie)
		for i := range ip {
			ip[i] ^= cookie[i]
		}
	}

	return netip.AddrPortFrom(netip.AddrFrom4(ip), port), nil
}
