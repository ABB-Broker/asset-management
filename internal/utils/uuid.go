// Package utils provides shared helpers used across handlers.
package utils

import (
	"crypto/rand"
	"fmt"
)

// NewUUID generates a random UUID v4 string in the standard
// xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx format using crypto/rand.
// It panics only if the OS entropy source is completely broken — that
// never happens in practice, so callers don't need to check errors.
func NewUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback: should never happen; satisfies the compiler.
		panic("utils.NewUUID: crypto/rand unavailable: " + err.Error())
	}
	// Set version 4 (random) and variant bits (RFC 4122).
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16],
	)
}
