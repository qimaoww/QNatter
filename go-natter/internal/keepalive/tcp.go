package keepalive

import (
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"

	"natter-openwrt/go-natter/internal/socketopts"
)

type TCPClient struct {
	Host      string
	Port      int
	Source    netip.AddrPort
	Interface string
	Timeout   time.Duration
	conn      net.Conn
}

func (c *TCPClient) KeepAlive() error {
	if c.conn == nil {
		conn, err := c.dial()
		if err != nil {
			return err
		}
		c.conn = conn
	}

	timeout := c.timeout()
	if err := c.conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		c.disconnect()
		return err
	}

	request := fmt.Sprintf(
		"HEAD /natter-keep-alive HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"User-Agent: curl/8.0.0 (Natter)\r\n"+
			"Accept: */*\r\n"+
			"Connection: keep-alive\r\n"+
			"\r\n",
		c.Host,
	)
	if _, err := io.WriteString(c.conn, request); err != nil {
		c.disconnect()
		return err
	}

	buf := make([]byte, 4096)
	received := false
	for {
		n, err := c.conn.Read(buf)
		if n > 0 {
			received = true
		}
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() && received {
				return nil
			}
			c.disconnect()
			return err
		}
		if n == 0 {
			c.disconnect()
			return io.ErrUnexpectedEOF
		}
	}
}

func (c *TCPClient) Close() error {
	if c.conn == nil {
		return nil
	}
	conn := c.conn
	c.conn = nil
	return conn.Close()
}

func (c *TCPClient) dial() (net.Conn, error) {
	network := socketopts.NetworkForSource("tcp", c.Source)
	dialer := net.Dialer{Timeout: c.timeout()}
	localAddr, err := socketopts.LocalAddr(network, c.Source)
	if err != nil {
		return nil, err
	}
	dialer.LocalAddr = localAddr
	dialer.Control = socketopts.Control(socketopts.Options{
		Interface: c.Interface,
		Reuse:     true,
	})
	return dialer.Dial(network, fmt.Sprintf("%s:%d", c.Host, c.Port))
}

func (c *TCPClient) disconnect() {
	_ = c.Close()
}

func (c *TCPClient) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return 3 * time.Second
}
