package keepalive

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

type UDPClient struct {
	Host    string
	Port    int
	Timeout time.Duration
	conn    net.Conn
}

func (c *UDPClient) KeepAlive() error {
	if c.conn == nil {
		conn, err := c.dial()
		if err != nil {
			return err
		}
		c.conn = conn
	}

	if err := c.conn.SetDeadline(time.Now().Add(c.timeout())); err != nil {
		c.disconnect()
		return err
	}
	if _, err := c.conn.Write(dnsKeepAliveQuery()); err != nil {
		c.disconnect()
		return err
	}

	buf := make([]byte, 1500)
	n, err := c.conn.Read(buf)
	if err != nil {
		c.disconnect()
		return err
	}
	if n == 0 {
		c.disconnect()
		return fmt.Errorf("keep-alive server closed connection")
	}
	return nil
}

func (c *UDPClient) Close() error {
	if c.conn == nil {
		return nil
	}
	conn := c.conn
	c.conn = nil
	return conn.Close()
}

func (c *UDPClient) dial() (net.Conn, error) {
	dialer := net.Dialer{Timeout: c.timeout()}
	return dialer.Dial("udp", fmt.Sprintf("%s:%d", c.Host, c.Port))
}

func (c *UDPClient) disconnect() {
	_ = c.Close()
}

func (c *UDPClient) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return 3 * time.Second
}

func dnsKeepAliveQuery() []byte {
	msg := make([]byte, 0, 34)
	txid := [2]byte{}
	_, _ = rand.Read(txid[:])
	msg = append(msg, txid[:]...)
	header := make([]byte, 10)
	binary.BigEndian.PutUint16(header[0:2], 0x0100)
	binary.BigEndian.PutUint16(header[2:4], 0x0001)
	msg = append(msg, header...)
	msg = append(msg, []byte("\x09keepalive\x06natter\x00")...)
	tail := make([]byte, 4)
	binary.BigEndian.PutUint16(tail[0:2], 0x0001)
	binary.BigEndian.PutUint16(tail[2:4], 0x0001)
	msg = append(msg, tail...)
	return msg
}
