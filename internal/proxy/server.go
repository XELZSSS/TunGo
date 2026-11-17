package proxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

const setupTimeout = 30 * time.Second

type Server struct {
	ln   net.Listener
	dial Dialer

	up, down atomic.Int64

	mu     sync.Mutex
	conns  []net.Conn
	closed bool
}

func Listen(addr string, dial Dialer) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &Server{ln: ln, dial: dial}
	go s.serve()
	return s, nil
}

func (s *Server) Addr() string { return s.ln.Addr().String() }

func (s *Server) Stats() (up, down int64) { return s.up.Load(), s.down.Load() }

func (s *Server) Close() error {
	err := s.ln.Close()

	s.mu.Lock()
	s.closed = true
	conns := s.conns
	s.conns = nil
	s.mu.Unlock()

	for _, c := range conns {
		c.Close()
	}
	return err
}

func (s *Server) serve() {
	fails := 0
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			if s.isClosed() {
				return
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			fails++
			if fails > 100 {
				return
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		fails = 0
		if !s.track(conn) {
			conn.Close()
			return
		}
		go s.handle(conn)
	}
}

func (s *Server) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Server) track(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.conns = append(s.conns, conn)
	return true
}

func (s *Server) untrack(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.conns {
		if c == conn {
			s.conns = append(s.conns[:i], s.conns[i+1:]...)
			return
		}
	}
}

func (s *Server) handle(conn net.Conn) {
	defer s.untrack(conn)
	if err := conn.SetDeadline(time.Now().Add(setupTimeout)); err != nil {
		conn.Close()
		return
	}
	br := bufio.NewReader(conn)
	first, err := br.Peek(1)
	if err != nil {
		conn.Close()
		return
	}
	if first[0] == socks5 {
		s.socks(conn, br)
		return
	}
	s.http(conn, br)
}

func (s *Server) socks(conn net.Conn, br *bufio.Reader) {
	target, err := s.negotiate(conn, br)
	if err != nil {
		conn.Close()
		return
	}
	defer target.Close()
	s.tunnel(conn, br, target)
}

func (s *Server) tunnel(conn net.Conn, br *bufio.Reader, target net.Conn) {
	done := make(chan struct{})
	go func() {
		s.forward(target, io.MultiReader(br, conn), &s.up)
		close(done)
	}()
	s.forward(conn, target, &s.down)
	conn.Close()
	target.Close()
	<-done
}

func (s *Server) negotiate(conn net.Conn, br *bufio.Reader) (net.Conn, error) {
	if err := conn.SetDeadline(time.Now().Add(setupTimeout)); err != nil {
		return nil, err
	}
	defer conn.SetDeadline(time.Time{})

	var greeting [2]byte
	if _, err := io.ReadFull(br, greeting[:]); err != nil {
		return nil, err
	}
	if greeting[0] != socks5 {
		return nil, fmt.Errorf("socks5: version %d", greeting[0])
	}
	methods := make([]byte, greeting[1])
	if _, err := io.ReadFull(br, methods); err != nil {
		return nil, err
	}

	if !slices.Contains(methods, byte(0)) {
		conn.Write([]byte{socks5, methodNone})
		return nil, errors.New("socks5: 客户端不支持免认证")
	}
	if _, err := conn.Write([]byte{socks5, 0}); err != nil {
		return nil, err
	}

	var request [3]byte
	if _, err := io.ReadFull(br, request[:]); err != nil {
		return nil, err
	}
	target, err := readAddr(br)
	if err != nil {
		return nil, err
	}
	if request[0] != socks5 {
		return nil, fmt.Errorf("socks5: version %d", request[0])
	}
	if request[1] != 1 {
		writeReply(conn, replyCommandNotSupported)
		return nil, errors.New("socks5: only CONNECT is supported")
	}

	remote, err := s.dial.DialContext(context.Background(), "tcp", target)
	if err != nil {
		writeReply(conn, replyHostUnreachable)
		return nil, err
	}
	if err := writeReply(conn, replyOK); err != nil {
		remote.Close()
		return nil, err
	}
	return remote, nil
}

func (s *Server) forward(dst net.Conn, src io.Reader, n *atomic.Int64) {
	io.Copy(counting{dst, n}, src)
	if cw, ok := dst.(interface{ CloseWrite() error }); ok {
		cw.CloseWrite()
	}
}

type counting struct {
	w io.Writer
	n *atomic.Int64
}

func (c counting) Write(p []byte) (int, error) {
	written, err := c.w.Write(p)
	c.n.Add(int64(written))
	return written, err
}
