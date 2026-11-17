package logs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const keep = 7

var (
	mu   sync.Mutex
	dir  string
	file *os.File
	day  string
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
			return
		}
		clear()
	}
	fmt.Fprintln(file, line)
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
	path := filepath.Join(dir, "tungo-"+now.Format("2006-01-02")+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if file != nil {
		file.Close()
	}
	file, day = f, now.Format("2006-01-02")
	return nil
}

func clear() {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -keep)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), "tungo-") || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}
