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

const (
	setupTimeout = 30 * time.Second
	idleTimeout  = 5 * time.Minute
)

type Server struct {
	ln   net.Listener
	dial Dialer

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	up, down atomic.Int64

	mu     sync.Mutex
	conns  map[net.Conn]struct{}
	closed bool
}

func Listen(addr string, dial Dialer) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{ln: ln, dial: dial, ctx: ctx, cancel: cancel, done: make(chan struct{}), conns: map[net.Conn]struct{}{}}
	go s.serve()
	return s, nil
}

func (s *Server) Addr() string { return s.ln.Addr().String() }

func (s *Server) Done() <-chan struct{} { return s.done }

func (s *Server) Stats() (up, down int64) { return s.up.Load(), s.down.Load() }

func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	conns := s.conns
	s.conns = nil
	s.mu.Unlock()

	err := s.ln.Close()
	s.cancel()
	for conn := range conns {
		conn.Close()
	}
	return err
}

func (s *Server) serve() {
	defer close(s.done)
	fails := 0
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			fails++
			if fails > 100 {
				s.cancel()
				s.ln.Close()
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

func (s *Server) track(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.conns[conn] = struct{}{}
	return true
}

func (s *Server) untrack(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, conn)
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
	defer s.untrack(target)
	defer target.Close()
	s.tunnel(conn, br, target)
}

func (s *Server) tunnel(conn net.Conn, br *bufio.Reader, target net.Conn) {
	renew := func() {
		deadline := time.Now().Add(idleTimeout)
		conn.SetDeadline(deadline)
		target.SetDeadline(deadline)
	}
	renew()
	done := make(chan struct{})
	go func() {
		s.forward(target, io.MultiReader(br, conn), &s.up, renew)
		close(done)
	}()
	s.forward(conn, target, &s.down, renew)
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

	if !slices.Contains(methods, byte(methodNone)) {
		conn.Write([]byte{socks5, methodNoAcceptable})
		return nil, errors.New("socks5: the client offers no acceptable method")
	}
	if _, err := conn.Write([]byte{socks5, methodNone}); err != nil {
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

	remote, err := s.dialContext(target)
	if err != nil {
		writeReply(conn, replyHostUnreachable)
		return nil, err
	}
	if err := writeReply(conn, replyOK); err != nil {
		s.untrack(remote)
		remote.Close()
		return nil, err
	}
	return remote, nil
}

func (s *Server) dialContext(target string) (net.Conn, error) {
	conn, err := s.dial.DialContext(s.ctx, "tcp", target)
	if err != nil {
		return nil, err
	}
	if !s.track(conn) {
		conn.Close()
		return nil, net.ErrClosed
	}
	return conn, nil
}

var buffers = sync.Pool{New: func() any {
	buffer := make([]byte, 32<<10)
	return &buffer
}}

func (s *Server) forward(dst net.Conn, src io.Reader, n *atomic.Int64, renew func()) {
	buffer := buffers.Get().(*[]byte)
	defer buffers.Put(buffer)
	for {
		read, err := src.Read(*buffer)
		if read > 0 {
			written, failed := dst.Write((*buffer)[:read])
			n.Add(int64(written))
			renew()
			if failed != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}
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
