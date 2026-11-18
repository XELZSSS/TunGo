package appdir

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"tungo/internal/i18n"
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

func ReadJSON(name string, v any) error {
	path, err := file(name)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf(i18n.T("%s 解析失败：%w"), name, err)
	}
	return nil
}

func WriteJSON(name string, v any) error {
	path, err := file(name)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteFile(path, data, 0o600)
}

func file(name string) (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, name), nil
}

func WriteFile(path string, data []byte, perm fs.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	fresh := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(fresh)
		return err
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		os.Remove(fresh)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(fresh)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(fresh)
		return err
	}
	if err := os.Rename(fresh, path); err != nil {
		os.Remove(fresh)
		return err
	}
	return nil
}
