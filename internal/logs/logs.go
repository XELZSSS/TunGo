package logs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	keep    = 7
	maxSize = 8 << 20
)

var (
	mu      sync.Mutex
	dir     string
	file    *os.File
	day     string
	size    int64
	lastErr string
)

func Setup(into string) error {
	mu.Lock()
	defer mu.Unlock()
	dir = into
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	clear()
	if err := open(time.Now()); err != nil {
		return err
	}
	return nil
}

func Write(line string) {
	mu.Lock()
	defer mu.Unlock()
	if dir == "" {
		return
	}
	now := time.Now()
	if file == nil || day != now.Format("2006-01-02") {
		if err := open(now); err != nil {
			report(err)
			return
		}
		clear()
	}
	text := line + "\n"
	if _, err := file.WriteString(text); err != nil {
		report(err)
		return
	}
	size += int64(len(text))
	if size > maxSize {
		if err := rotate(now); err != nil {
			report(err)
		}
	}
}

type Writer struct{}

func (Writer) Write(p []byte) (int, error) {
	Write(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

func Dir() string {
	mu.Lock()
	defer mu.Unlock()
	return dir
}

func open(now time.Time) error {
	name := "tungo-" + now.Format("2006-01-02") + ".log"
	if err := openFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND); err != nil {
		return err
	}
	day = now.Format("2006-01-02")
	return nil
}

func rotate(now time.Time) error {
	base := filepath.Join(dir, "tungo-"+now.Format("2006-01-02"))
	for i := 1; ; i++ {
		err := openFile(base+"."+strconv.Itoa(i)+".log", os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}
		day = now.Format("2006-01-02")
		return nil
	}
}

func openFile(path string, flag int) error {
	next, err := os.OpenFile(path, flag, 0o600)
	if err != nil {
		return err
	}
	info, err := next.Stat()
	if err != nil {
		next.Close()
		return err
	}
	if file != nil {
		file.Close()
	}
	file, size = next, info.Size()
	return nil
}

func report(err error) {
	message := err.Error()
	if message == lastErr {
		return
	}
	lastErr = message
	fmt.Fprintln(os.Stderr, "tungo: log:", err)
}

func clear() {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -keep).Format("2006-01-02")
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "tungo-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		date, _, _ := strings.Cut(strings.TrimSuffix(strings.TrimPrefix(name, "tungo-"), ".log"), ".")
		if len(date) != 10 {
			continue
		}
		if _, err := time.Parse("2006-01-02", date); err != nil {
			continue
		}
		if date < cutoff {
			os.Remove(filepath.Join(dir, name))
		}
	}
}
