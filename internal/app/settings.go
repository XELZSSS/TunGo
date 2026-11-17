package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"tungo/internal/appdir"
)

type settings struct {
	Skipped   map[string]string `json:"skipped"`
	Kind      string            `json:"kind,omitempty"`
	Theme     string            `json:"theme,omitempty"`
	Lang      string            `json:"lang,omitempty"`
	Proxy     string            `json:"proxy,omitempty"`
	Tun       bool              `json:"tun,omitempty"`
	Shortcuts map[string]string `json:"shortcuts,omitempty"`
}

func loadSettings() (settings, error) {
	out := settings{Skipped: map[string]string{}}
	path, err := settingsPath()
	if err != nil {
		return out, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, fmt.Errorf("settings.json 解析失败：%w", err)
	}
	if out.Skipped == nil {
		out.Skipped = map[string]string{}
	}
	return out, nil
}

func (s settings) save() error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	fresh := path + ".tmp"
	if err := os.WriteFile(fresh, data, 0o600); err != nil {
		return err
	}
	return os.Rename(fresh, path)
}

func settingsPath() (string, error) {
	d, err := appdir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "settings.json"), nil
}
