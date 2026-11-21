package core

import (
	"crypto/rand"
	"errors"
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
	"tungo/internal/i18n"
)

type Kind string

const (
	SingBox Kind = "sing-box"
	Xray    Kind = "xray"

	Auto Kind = "auto"
)

var Choices = []Kind{Auto, SingBox, Xray}

func PickKind(kind Kind) Kind {
	switch kind {
	case SingBox, Xray:
		return kind
	default:
		return SingBox
	}
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
	if n.Detour != nil && !Carries(kind, *n.Detour) {
		return false
	}
	return true
}

type Capabilities struct {
	Stats            bool
	CloseConnections bool
}

func (k Kind) Capabilities() Capabilities {
	sing := k == SingBox
	return Capabilities{Stats: sing, CloseConnections: sing}
}

const (
	startTimeout    = 15 * time.Second
	tunTimeout      = 45 * time.Second
	tunReadyTimeout = 25 * time.Second
	stopTimeout     = 5 * time.Second
)

var resources string

func SetResources(dir string) { resources = dir }

func CleanRun() error {
	run, err := appdir.Sub("run")
	if err != nil {
		return err
	}
	return os.RemoveAll(run)
}

func resourceBin() (string, error) {
	if resources == "" {
		return "", errors.New(i18n.T("资源目录还没定位"))
	}
	return filepath.Join(resources, "bin"), nil
}

func binDir() (string, error) {
	d, err := appdir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "bin"), nil
}

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
		if info, err := os.Stat(place); err == nil {
			if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
				continue
			}
			return place, nil
		}
	}
	if found, err := exec.LookPath(name); err == nil {
		return found, nil
	}
	return "", fmt.Errorf(i18n.T("找不到 %s"), name)
}

type Kernel struct {
	kind   Kind
	cmd    *exec.Cmd
	socks  string
	secret string
	path   string
	lines  *ring
	done   chan struct{}
	once   sync.Once
}

func launch(kind Kind, binary string, socks string, config []byte, timeout time.Duration) (*Kernel, error) {
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
		kind:  kind,
		cmd:   cmd,
		socks: socks,
		path:  path,
		lines: lines,
		done:  make(chan struct{}),
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
			return fmt.Errorf(i18n.T("已退出：%s"), k.lines.tail())
		default:
		}
		if time.Now().After(deadline) {
			return fmt.Errorf(i18n.T("没有起来：%s"), k.lines.tail())
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
	kind = PickKind(kind)

	if !Carries(kind, n) {
		return nil, fmt.Errorf(i18n.T("%s 带不了 %q"), kind, n.Protocol)
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
	tunName := ""
	if tun {
		timeout = tunTimeout
		if kind != SingBox {
			if err := EnsureWintun(binary); err != nil {
				return nil, err
			}
		}
		cleanTun()
		tunName = tunInterface
	}
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		clash := ""
		secret := ""
		if kind == SingBox {
			clash = "127.0.0.1:0"
			secret = rand.Text()
		}
		config, err := Config(kind, n, socks, policy, clash, secret, tunName)
		if err != nil {
			return nil, err
		}
		k, err := launch(kind, binary, socks, config, timeout)
		if err == nil {
			k.secret = secret
			if tunName != "" && !waitTun(k, tunName, tunReadyTimeout) {
				k.Stop()
				last = errors.New(i18n.T("TUN 网卡没接管"))
				cleanTun()
				continue
			}
			return k, nil
		}
		last = err
		if !tun || !retryable(err) {
			break
		}
		if tun {
			cleanTun()
		}
	}
	return nil, fmt.Errorf(i18n.T("%s 起不来：%w"), kind, last)
}

func retryable(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, text := range []string{"already exists", "element not found", "object already exists", "cannot create", "adapter"} {
		if strings.Contains(msg, text) {
			return true
		}
	}
	return false
}

func (k *Kernel) Kind() Kind { return k.kind }

func (k *Kernel) Done() <-chan struct{} { return k.done }

func (k *Kernel) exited() bool {
	select {
	case <-k.done:
		return true
	default:
		return false
	}
}

func (k *Kernel) SocksAddr() string { return k.socks }

func (k *Kernel) Output() string { return k.lines.tail() }

func (k *Kernel) Stop() {
	k.once.Do(func() {
		stop(k.cmd)
		select {
		case <-k.done:
		case <-time.After(stopTimeout):
			k.cmd.Process.Kill()
			<-k.done
		}
		os.Remove(k.path)
	})
}

type ring struct {
	mu      sync.Mutex
	keep    int
	lines   []string
	next    int
	clash   string
	pending string
}

func (r *ring) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	chunks := strings.Split(r.pending+string(p), "\n")
	r.pending = chunks[len(chunks)-1]
	for _, line := range chunks[:len(chunks)-1] {
		r.add(line)
	}
	return len(p), nil
}

func (r *ring) add(line string) {
	line = strings.TrimRight(line, "\r")
	if strings.TrimSpace(line) == "" {
		return
	}
	if addr := clashAddrIn(line); addr != "" {
		r.clash = addr
	}
	if len(r.lines) < r.keep {
		r.lines = append(r.lines, line)
		return
	}
	r.lines[r.next] = line
	r.next = (r.next + 1) % r.keep
}

func (r *ring) tail() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	ordered := make([]string, 0, len(r.lines)+1)
	if len(r.lines) < r.keep {
		ordered = append(ordered, r.lines...)
	} else {
		ordered = append(ordered, r.lines[r.next:]...)
		ordered = append(ordered, r.lines[:r.next]...)
	}
	if pending := strings.TrimRight(r.pending, "\r"); strings.TrimSpace(pending) != "" {
		ordered = append(ordered, pending)
	}
	return strings.Join(ordered, "\n")
}

func (r *ring) api() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if addr := clashAddrIn(strings.TrimRight(r.pending, "\r")); addr != "" {
		return addr
	}
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

func BinDir() (string, error) { return binDir() }
