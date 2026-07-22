// Package zfsname validates pool/dataset/snapshot names passed to zfs(8)/zpool(8).
// User-controlled names must never be placed before "--" or they can be parsed as flags
// (e.g. pool="-c,iostat-10s" → zpool status -c column scripts).
package zfsname

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// ErrInvalid indicates a rejected pool/dataset/snapshot name (client error).
var ErrInvalid = errors.New("invalid zfs name")

// Check rejects empty names, leading dashes (flag injection), and control characters.
func Check(kind, name string) error {
	if name == "" {
		return fmt.Errorf("%w: %s required", ErrInvalid, kind)
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("%w: %s must not start with '-'", ErrInvalid, kind)
	}
	for _, r := range name {
		if r == 0 || r == '\n' || r == '\r' || (unicode.IsControl(r) && r != '\t') {
			return fmt.Errorf("%w: %s contains control characters", ErrInvalid, kind)
		}
	}
	return nil
}

// Append appends "--" then name to args after Check succeeds.
func Append(args []string, kind, name string) ([]string, error) {
	if err := Check(kind, name); err != nil {
		return nil, err
	}
	return append(args, "--", name), nil
}

// Append2 appends "--", a, b after validating both names.
func Append2(args []string, kindA, a, kindB, b string) ([]string, error) {
	if err := Check(kindA, a); err != nil {
		return nil, err
	}
	if err := Check(kindB, b); err != nil {
		return nil, err
	}
	return append(args, "--", a, b), nil
}
