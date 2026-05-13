// Package totp provides RFC 6238 Time-based One-Time Password helpers.
package totp

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"time"
)

// base32Enc is the package-level Base-32 encoder (no padding) shared by all
// calls to Generate and Validate to avoid re-creating the object on every call.
var base32Enc = base32.StdEncoding.WithPadding(base32.NoPadding)

// devOTPBypass is the static code accepted in development when
// DEV_OTP_BYPASS is set in the environment. It is intentionally
// an unexported package-level var so tests can override it.
var devOTPBypass = os.Getenv("DEV_OTP_BYPASS")

// Validate checks whether code matches the current TOTP window (±1 step).
// The Base-32 key is decoded only once and reused for all three window checks.
// In development, if the DEV_OTP_BYPASS environment variable is set to a
// non-empty value, any code equal to that value is accepted immediately.
func Validate(secret, code string, now time.Time) bool {
	if len(code) != 6 {
		return false
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}

	// Dev-mode bypass: accept the static code without TOTP math.
	if devOTPBypass != "" && code == devOTPBypass {
		return true
	}

	key, err := base32Enc.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return false
	}
	for offset := -1; offset <= 1; offset++ {
		counter := uint64(now.Add(time.Duration(offset)*30*time.Second).Unix() / 30)
		if generateWithKey(key, counter) == code {
			return true
		}
	}
	return false
}

// Generate produces a 6-digit TOTP code for the given Base-32 secret and time.
func Generate(secret string, t time.Time) string {
	key, err := base32Enc.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}
	return generateWithKey(key, uint64(t.Unix()/30))
}

// generateWithKey is the hot path: it computes a 6-digit HOTP value for a
// pre-decoded HMAC key and a counter value.
func generateWithKey(key []byte, counter uint64) string {
	// Stack-allocated array avoids a heap allocation on every call.
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	h := hmac.New(sha1.New, key)
	h.Write(buf[:])
	hash := h.Sum(nil)
	offset := hash[len(hash)-1] & 0x0F
	value := (int(hash[offset])&0x7F)<<24 |
		(int(hash[offset+1])&0xFF)<<16 |
		(int(hash[offset+2])&0xFF)<<8 |
		(int(hash[offset+3]) & 0xFF)
	return fmt.Sprintf("%06d", value%1000000)
}
