package app

import "tungo/internal/appdir"

const settingsVersion = 1

type settings struct {
	Version   int               `json:"version"`
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
	RuleBase  string            `json:"ruleBase,omitempty"`
}

func loadSettings() (settings, error) {
	out := settings{Version: settingsVersion, Skipped: map[string]string{}}
	err := appdir.ReadJSON("settings.json", &out)
	if out.Version > settingsVersion {
		shortcuts, skipped := out.Shortcuts, out.Skipped
		out = settings{Version: settingsVersion, Skipped: skipped, Shortcuts: shortcuts}
	}
	if out.Skipped == nil {
		out.Skipped = map[string]string{}
	}
	return out, err
}

func (s settings) save() error {
	s.Version = settingsVersion
	return appdir.WriteJSON("settings.json", s)
}
