package appdir

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

const env = "TUNGO_DATA"

var (
	once sync.Once
	dir  string
	err  error
)

func Dir() (string, error) {
	once.Do(func() { dir, err = resolve() })
	return dir, err
}

func resolve() (string, error) {
	if d := os.Getenv(env); d != "" {
		return d, os.MkdirAll(d, 0o755)
	}
	base, err := home()
	if err != nil {
		return "", err
	}
	d := filepath.Join(base, "TunGo")
	return d, os.MkdirAll(d, 0o755)
}

func home() (string, error) {
	if runtime.GOOS == "windows" {
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return d, nil
		}
	}
	return os.UserConfigDir()
}

func Sub(name string) (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	d = filepath.Join(d, name)
	return d, os.MkdirAll(d, 0o755)
}
