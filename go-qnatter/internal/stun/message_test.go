package stun

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

func TestBuildBindingRequestMatchesQNatterWireFormat(t *testing.T) {
	txid := [12]byte{'N', 'A', 'T', 'R', 1, 2, 3, 4, 5, 6, 7, 8}

	msg := BuildBindingRequest(txid)

	if len(msg) != 20 {
		t.Fatalf("binding request length = %d, want 20", len(msg))
	}
	if got := binary.BigEndian.Uint16(msg[0:2]); got != 0x0001 {
		t.Fatalf("message type = %#x, want binding request", got)
	}
	if got := binary.BigEndian.Uint16(msg[2:4]); got != 0 {
		t.Fatalf("message length = %d, want 0", got)
	}
	if got := binary.BigEndian.Uint32(msg[4:8]); got != MagicCookie {
		t.Fatalf("magic cookie = %#x, want %#x", got, MagicCookie)
	}
	if got := string(msg[8:12]); got != "NATR" {
		t.Fatalf("transaction id prefix = %q, want NATR", got)
	}
	if got := [12]byte(msg[8:20]); got != txid {
		t.Fatalf("transaction id = %x, want %x", got, txid)
	}
}

func TestParseXORMappedAddress(t *testing.T) {
	txid := [12]byte{'N', 'A', 'T', 'R', 0, 0, 0, 0, 0, 0, 0, 0}
	response := make([]byte, 32)
	binary.BigEndian.PutUint16(response[0:2], 0x0101)
	binary.BigEndian.PutUint16(response[2:4], 12)
	binary.BigEndian.PutUint32(response[4:8], MagicCookie)
	copy(response[8:20], txid[:])
	binary.BigEndian.PutUint16(response[20:22], attrXORMappedAddress)
	binary.BigEndian.PutUint16(response[22:24], 8)
	response[24] = 0
	response[25] = 1
	binary.BigEndian.PutUint16(response[26:28], 54321^uint16(MagicCookie>>16))
	ip := netip.MustParseAddr("203.0.113.9").As4()
	cookieBytes := [4]byte{}
	binary.BigEndian.PutUint32(cookieBytes[:], MagicCookie)
	for i := range ip {
		response[28+i] = ip[i] ^ cookieBytes[i]
	}

	addr, err := ParseMappedAddress(response, txid)
	if err != nil {
		t.Fatalf("ParseMappedAddress returned error: %v", err)
	}
	if addr.Addr() != netip.MustParseAddr("203.0.113.9") {
		t.Fatalf("mapped IP = %s, want 203.0.113.9", addr.Addr())
	}
	if addr.Port() != 54321 {
		t.Fatalf("mapped port = %d, want 54321", addr.Port())
	}
}

func TestParseMappedAddressRejectsWrongTransaction(t *testing.T) {
	txid := [12]byte{'N', 'A', 'T', 'R', 1, 1, 1, 1, 1, 1, 1, 1}
	response := BuildBindingRequest([12]byte{'N', 'A', 'T', 'R', 2, 2, 2, 2, 2, 2, 2, 2})
	response[0] = 0x01
	response[1] = 0x01

	if _, err := ParseMappedAddress(response, txid); err == nil {
		t.Fatal("ParseMappedAddress accepted a response with the wrong transaction id")
	}
}
