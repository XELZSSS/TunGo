package proxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

func (s *Server) http(conn net.Conn, br *bufio.Reader) {
	if err := conn.SetDeadline(time.Now().Add(setupTimeout)); err != nil {
		conn.Close()
		return
	}
	req, err := http.ReadRequest(br)
	if err != nil {
		conn.Close()
		return
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return
	}

	if req.Method == http.MethodConnect {
		s.connect(conn, br, req.Host)
		return
	}

	if !req.URL.IsAbs() || req.URL.Host == "" {
		fmt.Fprint(conn, "HTTP/1.1 400 Bad Request\r\n\r\n")
		conn.Close()
		return
	}
	target, err := s.dial.DialContext(context.Background(), "tcp", withPort(req.URL.Host, "80"))
	if err != nil {
		fmt.Fprint(conn, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		conn.Close()
		return
	}
	defer target.Close()

	req.Header.Del("Proxy-Connection")
	req.Header.Del("Proxy-Authorization")
	req.Header.Set("Connection", "close")
	req.Close = true
	if err := req.Write(counting{target, &s.up}); err != nil {
		conn.Close()
		return
	}
	io.Copy(counting{conn, &s.down}, target)
	conn.Close()
}

func (s *Server) connect(conn net.Conn, br *bufio.Reader, host string) {
	target, err := s.dial.DialContext(context.Background(), "tcp", withPort(host, "443"))
	if err != nil {
		fmt.Fprint(conn, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		conn.Close()
		return
	}
	defer target.Close()
	if _, err := io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		conn.Close()
		return
	}
	s.tunnel(conn, br, target)
}

func withPort(host, port string) string {
	if h, p, err := net.SplitHostPort(host); err == nil {
		return net.JoinHostPort(h, p)
	}
	return net.JoinHostPort(strings.Trim(host, "[]"), port)
}
