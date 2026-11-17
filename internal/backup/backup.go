package backup

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"tungo/internal/appdir"
	"tungo/internal/i18n"
)

const maxEntry = 8 << 20

const keep = 10

var files = []string{
	"nodes.json", "subscription.json", "settings.json",
	"routing.json", "dns.json", "mirrors.txt",
}

func Dir() (string, error) {
	d, err := appdir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "backups"), nil
}

func Make() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "tungo-"+time.Now().Format("20060102-150405")+".zip")
	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer out.Close()
	z := zip.NewWriter(out)
	data, err := appdir.Dir()
	if err != nil {
		return "", err
	}
	for _, name := range files {
		source, err := os.Open(filepath.Join(data, name))
		if err != nil {
			continue
		}
		entry, err := z.Create(name)
		if err != nil {
			source.Close()
			return "", err
		}
		if _, err := io.Copy(entry, source); err != nil {
			source.Close()
			return "", err
		}
		source.Close()
	}
	if err := z.Close(); err != nil {
		return "", err
	}
	if err := prune(); err != nil {
		return "", err
	}
	return path, nil
}

func prune() error {
	paths, err := List()
	if err != nil {
		return err
	}
	if len(paths) <= keep {
		return nil
	}
	for _, old := range paths[keep:] {
		if err := os.Remove(old); err != nil {
			return err
		}
	}
	return nil
}

func List() ([]string, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".zip") {
			out = append(out, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out, nil
}

func Restore(path string) ([]string, error) {
	z, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	data, err := appdir.Dir()
	if err != nil {
		return nil, err
	}
	var restored []string
	for _, entry := range z.File {
		name := filepath.Base(entry.Name)
		if !known(name) {
			continue
		}
		source, err := entry.Open()
		if err != nil {
			return restored, err
		}
		target := filepath.Join(data, name)
		fresh := target + ".restore"
		out, err := os.Create(fresh)
		if err != nil {
			source.Close()
			return restored, err
		}
		written, err := io.Copy(out, io.LimitReader(source, maxEntry+1))
		source.Close()
		if err != nil {
			out.Close()
			os.Remove(fresh)
			return restored, err
		}
		if written > maxEntry {
			out.Close()
			os.Remove(fresh)
			return restored, fmt.Errorf(i18n.T("%s 太大，不是备份里的文件"), name)
		}
		if err := out.Close(); err != nil {
			os.Remove(fresh)
			return restored, err
		}
		if err := os.Chmod(fresh, 0o600); err != nil {
			os.Remove(fresh)
			return restored, err
		}
		if err := os.Rename(fresh, target); err != nil {
			os.Remove(fresh)
			return restored, err
		}
		restored = append(restored, name)
	}
	if len(restored) == 0 {
		return nil, errors.New(i18n.T("备份里没有认得的东西"))
	}
	return restored, nil
}

func known(name string) bool {
	for _, one := range files {
		if one == name {
			return true
		}
	}
	return false
}
