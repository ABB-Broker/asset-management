package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// validateTOTP checks whether code matches the current TOTP window (±1 step).
func validateTOTP(secret, code string, now time.Time) bool {
	if len(code) != 6 {
		return false
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}
	for offset := -1; offset <= 1; offset++ {
		if generateTOTP(secret, now.Add(time.Duration(offset)*30*time.Second)) == code {
			return true
		}
	}
	return false
}

// generateTOTP produces a 6-digit TOTP code for the given secret and time.
func generateTOTP(secret string, t time.Time) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}
	counter := uint64(t.Unix() / 30)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)
	h := hmac.New(sha1.New, key)
	h.Write(buf)
	hash := h.Sum(nil)
	offset := hash[len(hash)-1] & 0x0F
	value := (int(hash[offset])&0x7F)<<24 |
		(int(hash[offset+1])&0xFF)<<16 |
		(int(hash[offset+2])&0xFF)<<8 |
		(int(hash[offset+3]) & 0xFF)
	return fmt.Sprintf("%06d", value%1000000)
}
