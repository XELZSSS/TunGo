package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

func (s *Server) http(conn net.Conn, br *bufio.Reader) {
	req, err := http.ReadRequest(br)
	if err != nil {
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
	port := "80"
	if req.URL.Scheme == "https" {
		port = "443"
	}
	target, err := s.dialContext(withPort(req.URL.Host, port))
	if err != nil {
		fmt.Fprint(conn, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		conn.Close()
		return
	}
	defer s.untrack(target)
	defer target.Close()

	hopByHop(req.Header)
	req.Header.Set("Connection", "close")
	req.Close = true
	if err := req.Write(counting{target, &s.up}); err != nil {
		conn.Close()
		return
	}
	s.tunnel(conn, br, target)
}

func (s *Server) connect(conn net.Conn, br *bufio.Reader, host string) {
	target, err := s.dialContext(withPort(host, "443"))
	if err != nil {
		fmt.Fprint(conn, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		conn.Close()
		return
	}
	defer s.untrack(target)
	defer target.Close()
	if _, err := io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		conn.Close()
		return
	}
	s.tunnel(conn, br, target)
}

func hopByHop(header http.Header) {
	for _, name := range strings.Split(header.Get("Connection"), ",") {
		if name = strings.TrimSpace(name); name != "" {
			header.Del(name)
		}
	}
	for _, name := range []string{
		"Connection", "Proxy-Connection", "Proxy-Authorization", "Proxy-Authenticate",
		"Keep-Alive", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
	} {
		header.Del(name)
	}
}

func withPort(host, port string) string {
	if h, p, err := net.SplitHostPort(host); err == nil {
		return net.JoinHostPort(h, p)
	}
	return net.JoinHostPort(strings.Trim(host, "[]"), port)
}
