package i18n

import (
	_ "embed"
	"encoding/json"
	"regexp"
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

var formatVerb = regexp.MustCompile(`%(?:\[\d+\])?[-+#0 ]*(?:\d+|\*)?(?:\.(?:\d+|\*))?(?:\[\d+\])?[a-zA-Z]`)

func verbs(text string) string {
	matches := formatVerb.FindAllString(text, -1)
	out := make([]byte, 0, len(matches))
	for _, match := range matches {
		if match == "%%" {
			continue
		}
		out = append(out, match[len(match)-1])
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
