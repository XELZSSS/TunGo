package core

import "strings"

var mirror string

func SetMirror(prefix string) { mirror = strings.TrimSpace(prefix) }

func Mirror() string { return mirror }

func withMirror(url string) string {
	if mirror == "" {
		return url
	}
	if strings.HasSuffix(mirror, "/") {
		return mirror + url
	}
	return mirror + "/" + url
}
