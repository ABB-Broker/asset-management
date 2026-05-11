package models_test

import (
	"testing"

	"github.com/ABB-Broker/asset-management/internal/models"
)

// BenchmarkRandomToken measures the throughput of cryptographically random
// Base-32 token generation (used for session tokens).
func BenchmarkRandomToken(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = models.RandomToken()
	}
}
