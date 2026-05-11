package main

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	tmpl, err := templateForTest()
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}
	return &App{
		tmpl:          tmpl,
		adminUsername: "admin",
		adminHash:     hash,
		totpSecret:    "JBSWY3DPEHPK3PXP",
		sessions:      map[string]Session{},
		categories:    map[int]Category{},
		assets:        map[int]Asset{},
		nextCategory:  1,
		nextAsset:     1,
	}
}

func templateForTest() (*template.Template, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return template.ParseGlob(cwd + "/templates/*.html")
}

func TestLogin2FAFlow(t *testing.T) {
	app := newTestApp(t)

	form := url.Values{"username": {"admin"}, "password": {"secret123"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.handleLogin(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", rr.Code)
	}
	cookie := rr.Result().Cookies()[0]

	code := generateTOTP(app.totpSecret, time.Now())
	form2 := url.Values{"code": {code}}
	req2 := httptest.NewRequest(http.MethodPost, "/login/2fa", strings.NewReader(form2.Encode()))
	req2.AddCookie(cookie)
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr2 := httptest.NewRecorder()
	app.handleLogin2FA(rr2, req2)

	if rr2.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", rr2.Code)
	}
	sess, _, ok := app.pendingSession(req2)
	if ok || sess.Authenticated {
		t.Fatalf("session should not be pending after successful 2FA")
	}
}

func TestCategoryAndAssetCRUD(t *testing.T) {
	app := newTestApp(t)

	app.categories[1] = Category{ID: 1, Name: "IT"}
	app.nextCategory = 2

	createCategory := url.Values{"name": {"Office"}, "description": {"Office category"}}
	req := httptest.NewRequest(http.MethodPost, "/categories/create", strings.NewReader(createCategory.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.handleCreateCategory(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected category create redirect, got %d", rr.Code)
	}
	if len(app.categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(app.categories))
	}

	createAsset := url.Values{"name": {"Laptop"}, "serial_number": {"SN-1"}, "purchase_date": {"2026-01-01"}, "category_id": {"1"}}
	req2 := httptest.NewRequest(http.MethodPost, "/assets/create", strings.NewReader(createAsset.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr2 := httptest.NewRecorder()
	app.handleCreateAsset(rr2, req2)
	if rr2.Code != http.StatusSeeOther {
		t.Fatalf("expected asset create redirect, got %d", rr2.Code)
	}
	if len(app.assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(app.assets))
	}
}
