package proxy

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
)

const socks5 = 5

const (
	replyOK                  = 0
	replyHostUnreachable     = 4
	replyCommandNotSupported = 7

	methodNone         = 0
	methodNoAcceptable = 0xFF
)

var errBadAddr = errors.New("socks5: malformed address")

func readAddr(r io.Reader) (string, error) {
	var kind [1]byte
	if _, err := io.ReadFull(r, kind[:]); err != nil {
		return "", err
	}
	var host string
	switch kind[0] {
	case 1:
		var b [4]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return "", err
		}
		host = net.IP(b[:]).String()
	case 3:
		var n [1]byte
		if _, err := io.ReadFull(r, n[:]); err != nil {
			return "", err
		}
		if n[0] == 0 {
			return "", errBadAddr
		}
		b := make([]byte, n[0])
		if _, err := io.ReadFull(r, b); err != nil {
			return "", err
		}
		host = string(b)
	case 4:
		var b [16]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return "", err
		}
		host = net.IP(b[:]).String()
	default:
		return "", fmt.Errorf("socks5: address type %d", kind[0])
	}
	var port [2]byte
	if _, err := io.ReadFull(r, port[:]); err != nil {
		return "", err
	}
	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(port[:])))), nil
}

func writeAddr(w io.Writer, addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return err
	}
	var b []byte
	switch ip := net.ParseIP(host); {
	case ip != nil && ip.To4() != nil:
		b = append([]byte{1}, ip.To4()...)
	case ip != nil:
		b = append([]byte{4}, ip.To16()...)
	case len(host) <= 255:
		b = append([]byte{3, byte(len(host))}, host...)
	default:
		return errBadAddr
	}
	b = append(b, byte(p>>8), byte(p))
	_, err = w.Write(b)
	return err
}

func writeReply(w io.Writer, code byte) error {
	_, err := w.Write([]byte{socks5, code, 0, 1, 0, 0, 0, 0, 0, 0})
	return err
}
