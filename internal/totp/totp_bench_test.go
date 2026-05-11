package totp_test

import (
	"testing"
	"time"

	"github.com/ABB-Broker/asset-management/internal/totp"
)

const (
	benchSecret = "JBSWY3DPEHPK3PXP"
)

// BenchmarkGenerate measures the throughput of TOTP code generation.
// This covers Base-32 decode + HMAC-SHA1 + formatting on every call.
func BenchmarkGenerate(b *testing.B) {
	now := time.Now()
	b.ReportAllocs()

	for b.Loop() {
		_ = totp.Generate(benchSecret, now)
	}
}

// BenchmarkValidateValid measures Validate with a code that matches the
// current window — the hot path that decodes the key once and checks all
// three TOTP windows until a match is found.
func BenchmarkValidateValid(b *testing.B) {
	now := time.Now()
	code := totp.Generate(benchSecret, now)
	b.ReportAllocs()

	for b.Loop() {
		_ = totp.Validate(benchSecret, code, now)
	}
}

// BenchmarkValidateInvalid measures Validate with a code that never matches —
// all three windows are checked before returning false.
func BenchmarkValidateInvalid(b *testing.B) {
	now := time.Now()
	b.ReportAllocs()

	for b.Loop() {
		_ = totp.Validate(benchSecret, "000000", now)
	}
}
