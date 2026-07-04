package stun

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"syscall"
	"time"

	"qnatter-openwrt/go-qnatter/internal/socketopts"
)

type Server struct {
	Host string
	Port int
}

type Mapping struct {
	Inner  netip.AddrPort
	Outer  netip.AddrPort
	Server Server
}

var ErrNoServerAvailable = errors.New("no STUN server is available right now")

type ExchangeFunc func(context.Context, string, Server, netip.AddrPort, []byte) (netip.AddrPort, []byte, error)

type Client struct {
	Servers   []Server
	Source    netip.AddrPort
	UDP       bool
	TxID      func() ([12]byte, error)
	Do        ExchangeFunc
	Transport NetworkTransport

	initialSource netip.AddrPort
}

func (c *Client) GetMapping(ctx context.Context) (Mapping, error) {
	if len(c.Servers) == 0 {
		return Mapping{}, errors.New("STUN server list is empty")
	}
	if !c.initialSource.IsValid() {
		c.initialSource = c.Source
	}

	attempts := len(c.Servers)
	failures := make([]string, 0, attempts)
	localAddressUnavailable := false
	localDeviceUnavailable := false
	for i := 0; i < attempts; i++ {
		txid, err := c.transactionID()
		if err != nil {
			return Mapping{}, err
		}
		server := c.Servers[0]
		network := "tcp"
		if c.UDP {
			network = "udp"
		}

		inner, response, err := c.exchange()(ctx, network, server, c.Source, BuildBindingRequest(txid))
		if err != nil {
			if errors.Is(err, syscall.EADDRNOTAVAIL) {
				localAddressUnavailable = true
			}
			if errors.Is(err, syscall.ENODEV) {
				localDeviceUnavailable = true
			}
			failures = append(failures, fmt.Sprintf("%s://%s is unavailable: %v", network, server.Address(), err))
			c.rotateServer()
			continue
		}
		outer, err := ParseMappedAddress(response, txid)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s://%s is unavailable: %v", network, server.Address(), err))
			c.rotateServer()
			continue
		}
		c.Source = inner
		return Mapping{Inner: inner, Outer: outer, Server: server}, nil
	}

	if len(failures) == 0 {
		return Mapping{}, ErrNoServerAvailable
	}
	c.Source = c.initialSource
	if localAddressUnavailable {
		return Mapping{}, fmt.Errorf("%w: %w: %s", ErrNoServerAvailable, syscall.EADDRNOTAVAIL, strings.Join(failures, "; "))
	}
	if localDeviceUnavailable {
		return Mapping{}, fmt.Errorf("%w: %w: %s", ErrNoServerAvailable, syscall.ENODEV, strings.Join(failures, "; "))
	}
	return Mapping{}, fmt.Errorf("%w: %s", ErrNoServerAvailable, strings.Join(failures, "; "))
}

func (s Server) Address() string {
	return net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
}

func (c *Client) transactionID() ([12]byte, error) {
	if c.TxID != nil {
		return c.TxID()
	}
	return NewTransactionID()
}

func (c *Client) exchange() ExchangeFunc {
	if c.Do != nil {
		return c.Do
	}
	return c.Transport.Exchange
}

func (c *Client) rotateServer() {
	if len(c.Servers) < 2 {
		return
	}
	first := c.Servers[0]
	copy(c.Servers, c.Servers[1:])
	c.Servers[len(c.Servers)-1] = first
}

func NewTransactionID() ([12]byte, error) {
	var txid [12]byte
	copy(txid[:4], []byte("NATR"))
	if _, err := rand.Read(txid[4:]); err != nil {
		return [12]byte{}, err
	}
	return txid, nil
}

type NetworkTransport struct {
	Timeout   time.Duration
	Interface string
	Reuse     bool
}

func (t NetworkTransport) Exchange(ctx context.Context, network string, server Server, source netip.AddrPort, request []byte) (netip.AddrPort, []byte, error) {
	timeout := t.Timeout
	if timeout == 0 {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dialNetwork := socketopts.NetworkForSource(network, source)
	dialer := net.Dialer{Timeout: timeout}
	localAddr, err := socketopts.LocalAddr(dialNetwork, source)
	if err != nil {
		return netip.AddrPort{}, nil, err
	}
	dialer.LocalAddr = localAddr
	dialer.Control = socketopts.Control(socketopts.Options{
		Interface: t.Interface,
		Reuse:     t.Reuse,
	})

	conn, err := dialer.DialContext(ctx, dialNetwork, net.JoinHostPort(server.Host, strconv.Itoa(server.Port)))
	if err != nil {
		return netip.AddrPort{}, nil, err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if _, err := conn.Write(request); err != nil {
		return netip.AddrPort{}, nil, err
	}
	buf := make([]byte, 1500)
	n, err := conn.Read(buf)
	if err != nil {
		return netip.AddrPort{}, nil, err
	}
	inner, err := parseNetAddr(conn.LocalAddr())
	if err != nil {
		return netip.AddrPort{}, nil, err
	}
	return inner, buf[:n], nil
}

func parseNetAddr(addr net.Addr) (netip.AddrPort, error) {
	switch a := addr.(type) {
	case *net.TCPAddr:
		return netip.ParseAddrPort(net.JoinHostPort(a.IP.String(), strconv.Itoa(a.Port)))
	case *net.UDPAddr:
		return netip.ParseAddrPort(net.JoinHostPort(a.IP.String(), strconv.Itoa(a.Port)))
	default:
		return netip.AddrPort{}, fmt.Errorf("unsupported local address %T", addr)
	}
}
