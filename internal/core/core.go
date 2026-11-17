package core

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"tungo/internal/appdir"
)

type Kind string

const (
	SingBox Kind = "sing-box"
	Xray    Kind = "xray"

	Auto Kind = "auto"
)

var Kinds = []Kind{SingBox, Xray}

var Choices = append([]Kind{Auto}, Kinds...)

func PickKind(kind Kind, n Node) Kind {
	if kind != Auto {
		return kind
	}
	if n.Direct() {
		return SingBox
	}
	if Carries(Xray, n) {
		return Xray
	}
	return SingBox
}

func Carries(kind Kind, n Node) bool {
	if n.Direct() {
		return true
	}

	if n.Group() {
		for _, member := range n.Members {
			if !Carries(kind, member) {
				return false
			}
		}
		return len(n.Members) > 0
	}
	if !kind.Supports(n.Protocol) {
		return false
	}
	if n.Detour != nil && !n.Detour.Direct() {
		return kind.Supports(n.Detour.Protocol)
	}
	return true
}

const (
	startTimeout = 15 * time.Second
	tunTimeout   = 45 * time.Second
)

var resources string

func SetResources(dir string) { resources = dir }

func resourceBin() (string, error) {
	if resources == "" {
		return "", fmt.Errorf("资源目录还没定位")
	}
	return filepath.Join(resources, "bin"), nil
}

func binDir() (string, error) { return appdir.Sub("bin") }

func bins() []string {
	var dirs []string
	if bin, err := binDir(); err == nil {
		dirs = append(dirs, bin)
	}
	if bin, err := resourceBin(); err == nil {
		dirs = append(dirs, bin)
	}
	return dirs
}

func Binary(kind Kind) (string, error) {
	name := string(kind)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	for _, bin := range bins() {
		place := filepath.Join(bin, name)
		if _, err := os.Stat(place); err == nil {
			return place, nil
		}
	}
	if found, err := exec.LookPath(name); err == nil {
		return found, nil
	}
	return "", fmt.Errorf("找不到 %s", name)
}

type Kernel struct {
	kind    Kind
	cmd     *exec.Cmd
	socks   string
	path    string
	started time.Time
	lines   *ring
	done    chan struct{}
	once    sync.Once
}

func launch(kind Kind, binary string, n Node, socks string, config []byte, timeout time.Duration) (*Kernel, error) {
	run, err := appdir.Sub("run")
	if err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(run, string(kind)+"-*.json")
	if err != nil {
		return nil, err
	}
	path := file.Name()
	_, err = file.Write(config)
	file.Close()
	if err != nil {
		os.Remove(path)
		return nil, err
	}

	lines := &ring{keep: 20}
	cmd := exec.Command(binary, "run", "-c", path)
	cmd.Stdout = lines
	cmd.Stderr = lines
	prepare(cmd)
	if err := cmd.Start(); err != nil {
		os.Remove(path)
		return nil, err
	}

	follow(cmd)

	k := &Kernel{
		kind:    kind,
		cmd:     cmd,
		socks:   socks,
		path:    path,
		started: time.Now(),
		lines:   lines,
		done:    make(chan struct{}),
	}
	go func() {
		cmd.Wait()
		close(k.done)
	}()
	if err := k.wait(timeout); err != nil {
		k.Stop()
		return nil, err
	}
	return k, nil
}

func (k *Kernel) wait(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if k.up() {
			return nil
		}
		select {
		case <-k.done:
			return fmt.Errorf("已退出：%s", k.lines.tail())
		default:
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("没有起来：%s", k.lines.tail())
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (k *Kernel) clashAddr() string {
	if k.kind != SingBox {
		return ""
	}
	return k.lines.api()
}

func (k *Kernel) up() bool {
	addr := k.clashAddr()
	if addr == "" {
		if k.kind == SingBox {
			return false
		}
		addr = k.socks
	}
	if addr == "" {
		return true
	}
	conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func Start(kind Kind, n Node, socks string, tun bool) (*Kernel, error) {
	kind = PickKind(kind, n)

	if !Carries(kind, n) {
		return nil, fmt.Errorf("%s 带不了 %q", kind, n.Protocol)
	}
	binary, err := Binary(kind)
	if err != nil {
		return nil, err
	}
	policy, err := LoadPolicy()
	if err != nil {
		return nil, err
	}
	timeout := startTimeout
	if tun {
		timeout = tunTimeout
		if kind != SingBox {
			if err := EnsureWintun(binary); err != nil {
				return nil, err
			}
		}
		resetTun()
	}
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		clash := ""
		if kind == SingBox {
			clash = "127.0.0.1:0"
		}
		config, err := Config(kind, n, socks, policy, clash, tun)
		if err != nil {
			return nil, err
		}
		k, err := launch(kind, binary, n, socks, config, timeout)
		if err == nil {
			return k, nil
		}
		last = err
		if !retryable(err, tun) {
			break
		}
		if tun {
			resetTun()
		}
	}
	return nil, fmt.Errorf("%s 起不来：%w", kind, last)
}

func retryable(err error, tun bool) bool {
	if !tun {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "already exists") || strings.Contains(msg, "Element not found")
}

func (k *Kernel) Kind() Kind { return k.kind }

func (k *Kernel) Done() <-chan struct{} { return k.done }

func (k *Kernel) SocksAddr() string { return k.socks }

func (k *Kernel) Output() string { return k.lines.tail() }

func (k *Kernel) Stop() {
	k.once.Do(func() {
		k.cmd.Process.Kill()
		<-k.done
		os.Remove(k.path)
	})
}

type ring struct {
	mu    sync.Mutex
	keep  int
	lines []string
	clash string
}

func (r *ring) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, line := range strings.Split(string(p), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if addr := clashAddrIn(line); addr != "" {
			r.clash = addr
		}
		if len(r.lines) == r.keep {
			r.lines = r.lines[1:]
		}
		r.lines = append(r.lines, line)
	}
	return len(p), nil
}

func (r *ring) tail() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.lines, "\n")
}

func (r *ring) api() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.clash
}

func clashAddrIn(line string) string {
	const marker = "restful api listening at "
	i := strings.Index(line, marker)
	if i < 0 {
		return ""
	}
	return strings.TrimSpace(line[i+len(marker):])
}
