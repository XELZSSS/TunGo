package core

func singBoxECH(ech string) map[string]any {
	return map[string]any{
		"enabled": true,
		"config":  []any{"-----BEGIN ECH CONFIGS-----\n" + ech + "\n-----END ECH CONFIGS-----"},
	}
}
