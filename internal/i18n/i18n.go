package i18n

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sync/atomic"
)

//go:embed lang/en.json
var english []byte

type translations struct {
	lang string
	dict map[string]string
}

var current atomic.Pointer[translations]

var en = load()

func init() { Set("") }

func load() map[string]string {
	dict := map[string]string{}
	if err := json.Unmarshal(english, &dict); err != nil {
		panic("i18n：lang/en.json " + err.Error())
	}
	for zh, en := range dict {
		if verbs(zh) != verbs(en) {
			delete(dict, zh)
			fmt.Fprintln(os.Stderr, "i18n：格式动词对不上，跳过", zh)
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
	lang := "zh"
	if l == "en" {
		lang = "en"
	}
	current.Store(&translations{lang: lang, dict: en})
}

func Lang() string { return current.Load().lang }

func T(zh string) string {
	cur := current.Load()
	if cur.lang == "en" {
		if text, ok := cur.dict[zh]; ok {
			return text
		}
	}
	return zh
}
