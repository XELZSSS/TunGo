package backup

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"tungo/internal/appdir"
	"tungo/internal/i18n"
)

const maxEntry = 8 << 20

const keep = 10

var backupName = regexp.MustCompile(`^tungo-\d{8}-\d{6}\.\d{3}\.zip$`)
var files = []string{
	"nodes.json", "subscription.json", "settings.json",
	"routing.json", "dns.json",
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
	path := filepath.Join(dir, "tungo-"+time.Now().Format("20060102-150405.000")+".zip")
	fresh := path + ".tmp"
	out, err := os.Create(fresh)
	if err != nil {
		return "", err
	}
	fail := func(err error) (string, error) {
		out.Close()
		os.Remove(fresh)
		return "", err
	}
	z := zip.NewWriter(out)
	data, err := appdir.Dir()
	if err != nil {
		return fail(err)
	}
	for _, name := range files {
		source, err := os.Open(filepath.Join(data, name))
		if err != nil {
			continue
		}
		entry, err := z.Create(name)
		if err != nil {
			source.Close()
			return fail(err)
		}
		if _, err := io.Copy(entry, source); err != nil {
			source.Close()
			return fail(err)
		}
		source.Close()
	}
	if err := z.Close(); err != nil {
		return fail(err)
	}
	if err := out.Close(); err != nil {
		os.Remove(fresh)
		return "", err
	}
	if err := os.Rename(fresh, path); err != nil {
		os.Remove(fresh)
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
		if !entry.IsDir() && backupName.MatchString(entry.Name()) {
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
	var entries []*zip.File
	for _, entry := range z.File {
		name := filepath.Base(entry.Name)
		if !known(name) {
			continue
		}
		if entry.UncompressedSize64 > maxEntry {
			return nil, fmt.Errorf(i18n.T("%s 太大，不是备份里的文件"), name)
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, errors.New(i18n.T("备份里没有认得的东西"))
	}
	restored := make([]string, 0, len(entries))
	fresh := func(name string) string { return filepath.Join(data, name+".restore") }
	drop := func() {
		for _, name := range restored {
			os.Remove(fresh(name))
		}
	}
	for _, entry := range entries {
		name := filepath.Base(entry.Name)
		restored = append(restored, name)
		source, err := entry.Open()
		if err != nil {
			drop()
			return nil, err
		}
		out, err := os.Create(fresh(name))
		if err != nil {
			source.Close()
			drop()
			return nil, err
		}
		written, err := io.Copy(out, io.LimitReader(source, maxEntry+1))
		source.Close()
		if err == nil && written > maxEntry {
			err = fmt.Errorf(i18n.T("%s 太大，不是备份里的文件"), name)
		}
		if err == nil {
			err = out.Chmod(0o600)
		}
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			drop()
			return nil, err
		}
	}
	for at, name := range restored {
		if err := os.Rename(fresh(name), filepath.Join(data, name)); err != nil {
			restored = restored[at:]
			drop()
			return nil, err
		}
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
