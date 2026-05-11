package models_test

import (
	"testing"

	"github.com/ABB-Broker/asset-management/internal/models"
)

// BenchmarkRandomToken measures the throughput of cryptographically random
// Base-32 token generation (used for session tokens).
func BenchmarkRandomToken(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = models.RandomToken()
	}
}
