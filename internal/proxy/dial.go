package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

type Dialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

const handshakeTimeout = 10 * time.Second

type Upstream struct {
	Addr string
}

func (u *Upstream) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: handshakeTimeout}
	conn, err := d.DialContext(ctx, "tcp", u.Addr)
	if err != nil {
		return nil, err
	}
	if err := handshake(conn, addr); err != nil {
		conn.Close()
		return nil, fmt.Errorf("socks5 %s: %w", u.Addr, err)
	}
	return conn, nil
}

func handshake(conn net.Conn, addr string) error {
	if err := conn.SetDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		return err
	}
	defer conn.SetDeadline(time.Time{})

	if _, err := conn.Write([]byte{socks5, 1, 0}); err != nil {
		return err
	}
	var method [2]byte
	if _, err := io.ReadFull(conn, method[:]); err != nil {
		return err
	}
	if method[0] != socks5 || method[1] != 0 {
		return errors.New("the node asks for a method other than none")
	}
	if _, err := conn.Write([]byte{socks5, 1, 0}); err != nil {
		return err
	}
	if err := writeAddr(conn, addr); err != nil {
		return err
	}
	var head [3]byte
	if _, err := io.ReadFull(conn, head[:]); err != nil {
		return err
	}
	if head[0] != socks5 {
		return fmt.Errorf("version %d", head[0])
	}
	if head[1] != replyOK {
		return fmt.Errorf("the node replied %d", head[1])
	}

	_, err := readAddr(conn)
	return err
}
