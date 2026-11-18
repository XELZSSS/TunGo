package i18n

import (
	_ "embed"
	"encoding/json"
	"sync"
)

//go:embed lang/en.json
var english []byte

var (
	mu   sync.RWMutex
	lang = "zh"
)

var en = load()

func load() map[string]string {
	dict := map[string]string{}
	if err := json.Unmarshal(english, &dict); err != nil {
		panic("i18n：lang/en.json " + err.Error())
	}
	for zh, en := range dict {
		if verbs(zh) != verbs(en) {
			delete(dict, zh)
		}
	}
	return dict
}

func verbs(text string) string {
	var out []byte
	for i := 0; i < len(text); i++ {
		if text[i] != '%' {
			continue
		}
		i++
		if i >= len(text) {
			break
		}
		if text[i] == '%' {
			continue
		}
		out = append(out, text[i])
	}
	return string(out)
}

func Set(l string) {
	mu.Lock()
	defer mu.Unlock()
	if l == "en" {
		lang = "en"
		return
	}
	lang = "zh"
}

func Lang() string {
	mu.RLock()
	defer mu.RUnlock()
	return lang
}

func T(zh string) string {
	mu.RLock()
	defer mu.RUnlock()
	if lang == "en" {
		if text, ok := en[zh]; ok {
			return text
		}
	}
	return zh
}
