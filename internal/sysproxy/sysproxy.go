package sysproxy

type Mode string

const (
	ModeClear     Mode = "clear"
	ModeSet       Mode = "set"
	ModeUnchanged Mode = "unchanged"
	ModePac       Mode = "pac"
)

var Modes = []Mode{ModeClear, ModeSet, ModeUnchanged, ModePac}

func Clear() error { return clear() }
func Reset() error { return reset() }
