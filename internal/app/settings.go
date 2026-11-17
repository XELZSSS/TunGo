package app

import "tungo/internal/appdir"

type settings struct {
	Skipped   map[string]string `json:"skipped"`
	Kind      string            `json:"kind,omitempty"`
	Theme     string            `json:"theme,omitempty"`
	Lang      string            `json:"lang,omitempty"`
	Proxy     string            `json:"proxy,omitempty"`
	ProxyOn   string            `json:"proxyOn,omitempty"`
	Tun       bool              `json:"tun,omitempty"`
	Applied   string            `json:"applied,omitempty"`
	Port      int               `json:"port,omitempty"`
	Shortcuts map[string]string `json:"shortcuts,omitempty"`
}

func loadSettings() (settings, error) {
	out := settings{Skipped: map[string]string{}}
	if err := appdir.ReadJSON("settings.json", &out); err != nil {
		return out, err
	}
	if out.Skipped == nil {
		out.Skipped = map[string]string{}
	}
	return out, nil
}

func (s settings) save() error { return appdir.WriteJSON("settings.json", s) }
