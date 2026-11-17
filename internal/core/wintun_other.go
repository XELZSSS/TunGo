//go:build !windows

package core

func EnsureWintun(string) error { return nil }
