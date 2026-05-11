package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// newTestApp creates an isolated App and Fiber instance backed by an
// in-memory SQLite database. Prefork is disabled so tests run in a
// single process.
func newTestApp(t *testing.T) (*App, *fiber.App) {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&Category{}, &Asset{}, &User{}, &Session{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := Config{
		AdminUsername: "admin",
		AdminPassword: "secret123",
		TOTPSecret:    "JBSWY3DPEHPK3PXP",
		Prefork:       false,
	}
	handler := &App{db: db, cfg: cfg, adminHash: hash, translator: nil}
	fApp := newFiberApp(handler, nil)
	return handler, fApp
}

func TestLogin2FAFlow(t *testing.T) {
	handler, fApp := newTestApp(t)

	// Step 1: POST /login with valid credentials → redirect to /login/2fa
	form := url.Values{"username": {"admin"}, "password": {"secret123"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := fApp.Test(req)
	if err != nil {
		t.Fatalf("test /login: %v", err)
	}
	if resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected redirect (302) from /login, got %d: %s", resp.StatusCode, body)
	}

	var sessionCookie *http.Cookie
	for _, ck := range resp.Cookies() {
		if ck.Name == "session_id" {
			sessionCookie = ck
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session_id cookie set after successful login")
	}

	// Step 2: POST /login/2fa with valid TOTP code → redirect to /categories
	code := generateTOTP(handler.cfg.TOTPSecret, time.Now())
	form2 := url.Values{"code": {code}}
	req2 := httptest.NewRequest(http.MethodPost, "/login/2fa", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.AddCookie(sessionCookie)
	resp2, err := fApp.Test(req2)
	if err != nil {
		t.Fatalf("test /login/2fa: %v", err)
	}
	if resp2.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected redirect (302) from /login/2fa, got %d: %s", resp2.StatusCode, body)
	}

	// Verify the session is now fully authenticated in the database
	var sess Session
	handler.db.Where("token = ? AND authenticated = ?", sessionCookie.Value, true).First(&sess)
	if sess.ID == 0 {
		t.Fatal("session was not marked as authenticated in DB after successful 2FA")
	}
}

func TestLoginInvalidPassword(t *testing.T) {
	_, fApp := newTestApp(t)

	form := url.Values{"username": {"admin"}, "password": {"wrongpassword"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := fApp.Test(req)
	if err != nil {
		t.Fatalf("test /login: %v", err)
	}
	// Should re-render the login page (200), not redirect
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on bad credentials, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "invalid username or password") {
		t.Fatal("expected error message in response body")
	}
}

func TestCategoryAndAssetCRUD(t *testing.T) {
	handler, fApp := newTestApp(t)

	// Seed a valid authenticated session so CRUD routes are accessible
	expiry := time.Now().Add(sessionTTL)
	handler.db.Create(&Session{
		Token:         "test-token",
		Username:      "admin",
		Authenticated: true,
		ExpiresAt:     expiry,
	})
	authCookie := &http.Cookie{Name: "session_id", Value: "test-token"}

	// --- Category create ---
	form := url.Values{"name": {"IT"}, "description": {"IT equipment"}}
	req := httptest.NewRequest(http.MethodPost, "/categories/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(authCookie)
	resp, err := fApp.Test(req)
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected redirect after category create, got %d", resp.StatusCode)
	}

	var cat Category
	handler.db.First(&cat)
	if cat.ID == 0 || cat.Name != "IT" {
		t.Fatalf("category not persisted in DB: %+v", cat)
	}

	// --- Category update ---
	updateForm := url.Values{
		"id":          {strconv.FormatUint(uint64(cat.ID), 10)},
		"name":        {"IT Updated"},
		"description": {"Updated description"},
	}
	req2 := httptest.NewRequest(http.MethodPost, "/categories/update", strings.NewReader(updateForm.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.AddCookie(authCookie)
	resp2, err := fApp.Test(req2)
	if err != nil {
		t.Fatalf("update category: %v", err)
	}
	if resp2.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected redirect after category update, got %d", resp2.StatusCode)
	}
	handler.db.First(&cat, cat.ID)
	if cat.Name != "IT Updated" {
		t.Fatalf("category name not updated: got %q", cat.Name)
	}

	// --- Asset create ---
	assetForm := url.Values{
		"name":          {"Laptop"},
		"serial_number": {"SN-001"},
		"purchase_date": {"2026-01-01"},
		"category_id":   {strconv.FormatUint(uint64(cat.ID), 10)},
	}
	req3 := httptest.NewRequest(http.MethodPost, "/assets/create", strings.NewReader(assetForm.Encode()))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req3.AddCookie(authCookie)
	resp3, err := fApp.Test(req3)
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if resp3.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected redirect after asset create, got %d", resp3.StatusCode)
	}
	var asset Asset
	handler.db.Preload("Category").First(&asset)
	if asset.ID == 0 || asset.Name != "Laptop" || asset.CategoryID != cat.ID {
		t.Fatalf("asset not persisted correctly: %+v", asset)
	}

	// --- Asset delete ---
	delForm := url.Values{"id": {strconv.FormatUint(uint64(asset.ID), 10)}}
	req4 := httptest.NewRequest(http.MethodPost, "/assets/delete", strings.NewReader(delForm.Encode()))
	req4.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req4.AddCookie(authCookie)
	resp4, err := fApp.Test(req4)
	if err != nil {
		t.Fatalf("delete asset: %v", err)
	}
	if resp4.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected redirect after asset delete, got %d", resp4.StatusCode)
	}
	var count int64
	handler.db.Model(&Asset{}).Where("id = ?", asset.ID).Count(&count)
	if count != 0 {
		t.Fatalf("asset still present in DB after delete")
	}
}
