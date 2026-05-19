package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ABB-Broker/asset-management/internal/models"
)

// ─────────────────────────────────────────────────────────────────────────────
// Benchmarks — pure HTTP round-trip via fiber.Test
// ─────────────────────────────────────────────────────────────────────────────

// BenchmarkLoginGet measures rendering the unauthenticated login page.
func BenchmarkLoginGet(b *testing.B) {
	_, fApp := newTestApp(b)
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	b.ReportAllocs()

	for b.Loop() {
		resp, err := fApp.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkLoginPostInvalidPassword measures a failed login attempt
// (covers form parse + bcrypt.CompareHashAndPassword + template render).
func BenchmarkLoginPostInvalidPassword(b *testing.B) {
	_, fApp := newTestApp(b)
	body := url.Values{"username": {"admin"}, "password": {"wrong"}}.Encode()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := fApp.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkCategoriesIndex measures the categories list page
// (DB SELECT + template render) with 10 pre-seeded rows.
func BenchmarkCategoriesIndex(b *testing.B) {
	h, fApp := newTestApp(b)

	const token = "bench-cat-token"
	h.DB.Create(&models.Session{
		Token:         token,
		Username:      "admin",
		Authenticated: true,
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	})

	for i := range 10 {
		h.DB.Create(&models.Category{
			Name:        fmt.Sprintf("Category %d", i),
			Description: "benchmark seed",
		})
	}

	authCookie := &http.Cookie{Name: "session_id", Value: token}
	req := httptest.NewRequest(http.MethodGet, "/categories", nil)
	req.AddCookie(authCookie)

	b.ReportAllocs()

	for b.Loop() {
		resp, err := fApp.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkCategoriesCreate measures the full category creation path
// (form parse + DB INSERT + redirect).
func BenchmarkCategoriesCreate(b *testing.B) {
	h, fApp := newTestApp(b)
	const token = "bench-create-cat"
	h.DB.Create(&models.Session{
		Token:         token,
		Username:      "admin",
		Authenticated: true,
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	})
	authCookie := &http.Cookie{Name: "session_id", Value: token}

	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		body := url.Values{
			"name":        {fmt.Sprintf("Cat%d", i)},
			"description": {"benchmark"},
		}.Encode()
		req := httptest.NewRequest(http.MethodPost, "/categories/create", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(authCookie)
		resp, err := fApp.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkAssetsIndex measures the asset list page
// (DB SELECT with Preload("Category") + template render) with 10 pre-seeded rows.
func BenchmarkAssetsIndex(b *testing.B) {
	h, fApp := newTestApp(b)
	const token = "bench-assets-token"
	h.DB.Create(&models.Session{
		Token:         token,
		Username:      "admin",
		Authenticated: true,
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	})

	cat := models.Category{Name: "BenchCat", Description: "benchmark"}
	h.DB.Create(&cat)

	for i := range 10 {
		h.DB.Create(&models.Asset{
			Name:         fmt.Sprintf("Asset %d", i),
			CategoryID:   cat.ID,
			SerialNumber: fmt.Sprintf("SN-%d", i),
			PurchaseDate: "2026-01-01",
		})
	}

	authCookie := &http.Cookie{Name: "session_id", Value: token}
	req := httptest.NewRequest(http.MethodGet, "/assets", nil)
	req.AddCookie(authCookie)

	b.ReportAllocs()

	for b.Loop() {
		resp, err := fApp.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkUsersIndex measures the user list page
// (DB SELECT + template render) with 10 pre-seeded rows.
func BenchmarkUsersIndex(b *testing.B) {
	h, fApp := newTestApp(b)
	const token = "bench-users-token"
	h.DB.Create(&models.Session{
		Token:         token,
		Username:      "admin",
		Authenticated: true,
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	})

	for i := range 10 {
		h.DB.Create(&models.User{
			Username: fmt.Sprintf("user%d", i),
			Email:    fmt.Sprintf("user%d@example.com", i),
			FullName: fmt.Sprintf("User %d", i),
			Role:     "viewer",
			Active:   true,
		})
	}

	authCookie := &http.Cookie{Name: "session_id", Value: token}
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.AddCookie(authCookie)

	b.ReportAllocs()

	for b.Loop() {
		resp, err := fApp.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkUsersCreate measures user creation
// (form parse + role validation + DB INSERT with unique-index check + redirect).
func BenchmarkUsersCreate(b *testing.B) {
	h, fApp := newTestApp(b)
	const token = "bench-create-user"
	h.DB.Create(&models.Session{
		Token:         token,
		Username:      "admin",
		Authenticated: true,
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	})
	authCookie := &http.Cookie{Name: "session_id", Value: token}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		body := url.Values{
			"username":  {fmt.Sprintf("benchuser%d", i)},
			"email":     {fmt.Sprintf("bench%d@example.com", i)},
			"full_name": {"Bench User"},
			"role":      {"viewer"},
			"active":    {"true"},
		}.Encode()
		req := httptest.NewRequest(http.MethodPost, "/users/create", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(authCookie)
		resp, err := fApp.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}
