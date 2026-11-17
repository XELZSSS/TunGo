package i18n

import (
	_ "embed"
	"encoding/json"
)

//go:embed lang/en.json
var english []byte

var lang = "zh"

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
	for i := 0; i+1 < len(text); i++ {
		if text[i] != '%' {
			continue
		}
		i++
		out = append(out, text[i])
	}
	return string(out)
}

func Set(l string) {
	if l == "en" {
		lang = "en"
		return
	}
	lang = "zh"
}

func Lang() string { return lang }

func T(zh string) string {
	if lang == "en" {
		if text, ok := en[zh]; ok {
			return text
		}
	}
	return zh
}
