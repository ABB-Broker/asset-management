package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/ABB-Broker/asset-management/internal/config"
	"github.com/ABB-Broker/asset-management/internal/handlers"
	"github.com/ABB-Broker/asset-management/internal/models"
	"github.com/ABB-Broker/asset-management/internal/utils"
)

// sessionTTL mirrors the constant from the handlers package.
const sessionTTL = 24 * time.Hour

// minimalPNG is a 1×1 transparent PNG encoded as a data-URI, used to test
// signature embedding without requiring real canvas output.
const minimalPNG = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

// ─── Test helpers ─────────────────────────────────────────────────────────────

// newTestApp creates an isolated App and Fiber instance backed by an
// in-memory SQLite database. It seeds a single admin user and returns both
// the App (for direct DB access) and the Fiber app (for HTTP tests).
func newTestApp(t testing.TB) (*handlers.App, *fiber.App) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.LocationPhotos{},
		&models.Location{},
		&models.Category{},
		&models.AssetPhotos{},
		&models.Asset{},
		&models.User{},
		&models.Assignee{},
		&models.LendingLog{},
		&models.HandoverForm{},
		&models.Session{},
		&models.PasswordSetToken{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash seed password: %v", err)
	}
	user := models.User{Username: "admin", Password: string(hashedPassword), Active: true, Role: "admin"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	cfg := config.Config{
		AdminUsername: "admin",
		AdminPassword: "secret123",
		TOTPSecret:    "JBSWY3DPEHPK3PXP",
		Prefork:       false,
	}
	h := &handlers.App{DB: db, Cfg: cfg, AdminHash: hashedPassword, Translator: nil}
	return h, newFiberApp(h, nil)
}

// seedSession inserts a fully-authenticated session and returns the cookie.
func seedSession(t testing.TB, h *handlers.App) *http.Cookie {
	t.Helper()
	token := fmt.Sprintf("test-token-%d", time.Now().UnixNano())
	h.DB.Create(&models.Session{
		Token:         token,
		Username:      "admin",
		Authenticated: true,
		ExpiresAt:     time.Now().Add(sessionTTL),
	})
	return &http.Cookie{Name: "session_id", Value: token}
}

// seedCategory creates a Category row and returns it.
func seedCategory(t testing.TB, h *handlers.App, name string) models.Category {
	t.Helper()
	cat := models.Category{Name: name, Description: name + " desc"}
	if err := h.DB.Create(&cat).Error; err != nil {
		t.Fatalf("seed category: %v", err)
	}
	return cat
}

// seedLocation creates a Location row and returns it.
func seedLocation(t testing.TB, h *handlers.App, name string) models.Location {
	t.Helper()
	loc := models.Location{LocationName: name, Description: name + " desc"}
	if err := h.DB.Create(&loc).Error; err != nil {
		t.Fatalf("seed location: %v", err)
	}
	return loc
}

// seedAsset creates an Asset row and returns it.
func seedAsset(t testing.TB, h *handlers.App, name, assetType string, catID uint, locID *uint) models.Asset {
	t.Helper()
	asset := models.Asset{
		Name:          name,
		AssetType:     assetType,
		CategoryID:    catID,
		LocationID:    locID,
		SerialNumber:  "SN-" + name,
		PurchaseDate:  "2026-01-01",
		PurchasePrice: 1000000,
	}
	if err := h.DB.Create(&asset).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return asset
}

// seedAssignee creates an Assignee row and returns it.
func seedAssignee(t testing.TB, h *handlers.App, name, email string) models.Assignee {
	t.Helper()
	a := models.Assignee{FullName: name, Email: email, PhoneNumber: "08123456789"}
	if err := h.DB.Create(&a).Error; err != nil {
		t.Fatalf("seed assignee: %v", err)
	}
	return a
}

// seedLendingLog creates a LendingLog + HandoverForm and returns both.
func seedLendingLog(t testing.TB, h *handlers.App, assetID, assigneeID uint) (models.LendingLog, models.HandoverForm) {
	t.Helper()
	log := models.LendingLog{
		AssetID:    assetID,
		AssigneeID: assigneeID,
		LentAt:     time.Now(),
		Status:     "pending_signature",
	}
	if err := h.DB.Create(&log).Error; err != nil {
		t.Fatalf("seed lending log: %v", err)
	}
	now := time.Now()
	form := models.HandoverForm{
		LendingLogID: log.ID,
		SentAt:       &now,
		Status:       "sent",
	}
	if err := h.DB.Create(&form).Error; err != nil {
		t.Fatalf("seed handover form: %v", err)
	}
	return log, form
}

// postForm is a test helper that fires a POST with url-encoded form data.
func postForm(t testing.TB, fApp *fiber.App, path string, values url.Values, cookie *http.Cookie) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := fApp.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

// getRequest fires a GET and returns the response.
func getRequest(t testing.TB, fApp *fiber.App, path string, cookie *http.Cookie) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := fApp.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

// assertRedirect checks that a response is a 303 See Other.
func assertRedirect(t testing.TB, resp *http.Response) {
	t.Helper()
	if resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected redirect (303), got %d: %s", resp.StatusCode, body)
	}
}

// assertStatus checks that a response has the expected status code.
func assertStatus(t testing.TB, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status %d, got %d: %s", want, resp.StatusCode, body)
	}
}

// ─── Auth tests ───────────────────────────────────────────────────────────────

func TestLogin2FAFlow(t *testing.T) {
	h, fApp := newTestApp(t)

	// Step 1: POST /login → redirect to /login/2fa and get session cookie.
	resp := postForm(t, fApp, "/login",
		url.Values{"username": {"admin"}, "password": {"secret123"}}, nil)
	assertRedirect(t, resp)

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

	// Step 2: POST /login/2fa with valid TOTP code → redirect to /categories.
	otpCode := "123456"
	h.DB.Create(&models.EmailOTP{
		Username:  "admin",
		Code:      otpCode,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	})
	resp2 := postForm(t, fApp, "/login/2fa",
		url.Values{"code": {otpCode}}, sessionCookie)
	assertRedirect(t, resp2)

	// Verify the session is fully authenticated in the database.
	var sess models.Session
	h.DB.Where("token = ? AND authenticated = ?", sessionCookie.Value, true).First(&sess)
	if sess.ID == 0 {
		t.Fatal("session was not marked as authenticated in DB after successful 2FA")
	}
}

func TestLoginInvalidPassword(t *testing.T) {
	_, fApp := newTestApp(t)

	resp := postForm(t, fApp, "/login",
		url.Values{"username": {"admin"}, "password": {"wrongpassword"}}, nil)
	assertStatus(t, resp, http.StatusOK)

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "invalid username or password") {
		t.Fatal("expected 'invalid username or password' error in response body")
	}
}

func TestLoginMissingFields(t *testing.T) {
	_, fApp := newTestApp(t)

	cases := []struct {
		name   string
		values url.Values
	}{
		{"missing username", url.Values{"password": {"secret123"}}},
		{"missing password", url.Values{"username": {"admin"}}},
		{"empty body", url.Values{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postForm(t, fApp, "/login", tc.values, nil)
			// Should not redirect to the protected area.
			if resp.StatusCode == http.StatusSeeOther {
				loc := resp.Header.Get("Location")
				if !strings.Contains(loc, "2fa") {
					t.Fatalf("unexpected redirect to %q on missing credentials", loc)
				}
			}
		})
	}
}

func TestLogin2FAInvalidCode(t *testing.T) {
	_, fApp := newTestApp(t)

	// First get a pending-2FA session.
	resp := postForm(t, fApp, "/login",
		url.Values{"username": {"admin"}, "password": {"secret123"}}, nil)
	assertRedirect(t, resp)

	var sessionCookie *http.Cookie
	for _, ck := range resp.Cookies() {
		if ck.Name == "session_id" {
			sessionCookie = ck
		}
	}

	// POST bad code → should re-render 2FA page (200) or redirect back.
	resp2 := postForm(t, fApp, "/login/2fa",
		url.Values{"code": {"000000"}}, sessionCookie)
	if resp2.StatusCode == http.StatusSeeOther {
		loc := resp2.Header.Get("Location")
		if strings.Contains(loc, "/categories") || strings.Contains(loc, "/assets") {
			t.Fatal("bad TOTP code should not grant access")
		}
	}
}

func TestUnauthenticatedRedirect(t *testing.T) {
	_, fApp := newTestApp(t)

	protectedPaths := []string{
		"/categories",
		"/assets",
		"/locations",
		"/users",
		"/assignees",
	}
	for _, path := range protectedPaths {
		t.Run(path, func(t *testing.T) {
			resp := getRequest(t, fApp, path, nil)
			if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("%s: expected redirect or 401, got %d", path, resp.StatusCode)
			}
			if resp.StatusCode == http.StatusSeeOther {
				loc := resp.Header.Get("Location")
				if !strings.Contains(loc, "login") {
					t.Fatalf("%s: redirect should go to login, got %q", path, loc)
				}
			}
		})
	}
}

func TestLogout(t *testing.T) {
	h, fApp := newTestApp(t)
	cookie := seedSession(t, h)

	resp := getRequest(t, fApp, "/logout", cookie)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected redirect after logout, got %d", resp.StatusCode)
	}

	// Session should be gone from DB.
	var sess models.Session
	h.DB.Where("token = ?", cookie.Value).First(&sess)
	if sess.ID != 0 {
		t.Fatal("session still exists in DB after logout")
	}
}

// ─── Category tests ───────────────────────────────────────────────────────────

func TestCategoryCRUD(t *testing.T) {
	h, fApp := newTestApp(t)
	cookie := seedSession(t, h)

	// Create
	resp := postForm(t, fApp, "/categories/create",
		url.Values{"name": {"IT"}, "description": {"IT equipment"}}, cookie)
	assertRedirect(t, resp)

	var cat models.Category
	h.DB.First(&cat)
	if cat.Name != "IT" {
		t.Fatalf("category not persisted, got: %+v", cat)
	}

	// Update
	resp2 := postForm(t, fApp, "/categories/update", url.Values{
		"id":          {strconv.FormatUint(uint64(cat.ID), 10)},
		"name":        {"IT Updated"},
		"description": {"Updated desc"},
	}, cookie)
	assertRedirect(t, resp2)
	h.DB.First(&cat, cat.ID)
	if cat.Name != "IT Updated" {
		t.Fatalf("category name not updated: %q", cat.Name)
	}

	// Delete
	resp3 := postForm(t, fApp, "/categories/delete",
		url.Values{"id": {strconv.FormatUint(uint64(cat.ID), 10)}}, cookie)
	assertRedirect(t, resp3)
	var count int64
	h.DB.Model(&models.Category{}).Where("id = ?", cat.ID).Count(&count)
	if count != 0 {
		t.Fatal("category still present after delete")
	}
}

func TestCategoryCreateMissingName(t *testing.T) {
	h, fApp := newTestApp(t)
	cookie := seedSession(t, h)

	resp := postForm(t, fApp, "/categories/create",
		url.Values{"name": {""}, "description": {"no name"}}, cookie)
	// Should redirect with an error, not create a row.
	assertRedirect(t, resp)
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "error") {
		t.Fatalf("expected error redirect, got %q", loc)
	}
	var count int64
	h.DB.Model(&models.Category{}).Count(&count)
	if count != 0 {
		t.Fatal("category should not be created without a name")
	}
}

// ─── Location tests ───────────────────────────────────────────────────────────

func TestLocationCRUD(t *testing.T) {
	h, fApp := newTestApp(t)
	cookie := seedSession(t, h)

	// Create
	resp := postForm(t, fApp, "/locations/create",
		url.Values{"name": {"Server Room"}, "description": {"Main DC"}}, cookie)
	assertRedirect(t, resp)
	loc := resp.Header.Get("Location")
	if strings.Contains(loc, "error") {
		t.Fatalf("unexpected error creating location: %q", loc)
	}

	var location models.Location
	h.DB.First(&location)
	if location.LocationName != "Server Room" {
		t.Fatalf("location not persisted: %+v", location)
	}
	if location.LocationUUID == "" {
		t.Fatal("location UUID not generated")
	}

	// Update
	resp2 := postForm(t, fApp, "/locations/update", url.Values{
		"id":          {strconv.FormatUint(uint64(location.ID), 10)},
		"name":        {"Data Center"},
		"description": {"Primary DC"},
	}, cookie)
	assertRedirect(t, resp2)
	h.DB.First(&location, location.ID)
	if location.LocationName != "Data Center" {
		t.Fatalf("location name not updated: %q", location.LocationName)
	}

	// Delete
	resp3 := postForm(t, fApp, "/locations/delete",
		url.Values{"id": {strconv.FormatUint(uint64(location.ID), 10)}}, cookie)
	assertRedirect(t, resp3)
	var count int64
	h.DB.Model(&models.Location{}).Where("id = ?", location.ID).Count(&count)
	if count != 0 {
		t.Fatal("location still present after delete")
	}
}

func TestLocationCreateMissingName(t *testing.T) {
	h, fApp := newTestApp(t)
	cookie := seedSession(t, h)

	resp := postForm(t, fApp, "/locations/create",
		url.Values{"name": {""}, "description": {"no name"}}, cookie)
	assertRedirect(t, resp)
	if !strings.Contains(resp.Header.Get("Location"), "error") {
		t.Fatal("expected error redirect on missing location name")
	}
}

// ─── Asset tests ──────────────────────────────────────────────────────────────

func TestAssetCRUD(t *testing.T) {
	h, fApp := newTestApp(t)
	cookie := seedSession(t, h)

	cat := seedCategory(t, h, "Electronics")
	loc := seedLocation(t, h, "Warehouse")

	catID := strconv.FormatUint(uint64(cat.ID), 10)
	locID := strconv.FormatUint(uint64(loc.ID), 10)

	// Create fixed asset
	resp := postForm(t, fApp, "/assets/create", url.Values{
		"name":           {"Desktop PC"},
		"serial_number":  {"SN-PC-001"},
		"purchase_date":  {"2026-03-01"},
		"purchase_price": {"8000000"},
		"category_id":    {catID},
		"asset_type":     {"fixed"},
		"location_id":    {locID},
	}, cookie)
	assertRedirect(t, resp)

	var asset models.Asset
	h.DB.Preload("Category").First(&asset)
	if asset.Name != "Desktop PC" || asset.CategoryID != cat.ID {
		t.Fatalf("asset not persisted: %+v", asset)
	}
	if asset.AssetUUID == "" {
		t.Fatal("asset UUID not generated")
	}
	if asset.AssetType != "fixed" {
		t.Fatalf("expected fixed, got %q", asset.AssetType)
	}

	// Update
	resp2 := postForm(t, fApp, "/assets/update", url.Values{
		"id":             {strconv.FormatUint(uint64(asset.ID), 10)},
		"name":           {"Desktop PC v2"},
		"serial_number":  {"SN-PC-002"},
		"purchase_date":  {"2026-04-01"},
		"purchase_price": {"9000000"},
		"category_id":    {catID},
		"asset_type":     {"movable"},
		"location_id":    {locID},
	}, cookie)
	assertRedirect(t, resp2)
	h.DB.First(&asset, asset.ID)
	if asset.Name != "Desktop PC v2" {
		t.Fatalf("asset name not updated: %q", asset.Name)
	}
	if asset.AssetType != "movable" {
		t.Fatalf("asset type not updated: %q", asset.AssetType)
	}

	// Delete
	resp3 := postForm(t, fApp, "/assets/delete",
		url.Values{"id": {strconv.FormatUint(uint64(asset.ID), 10)}}, cookie)
	assertRedirect(t, resp3)
	var count int64
	h.DB.Model(&models.Asset{}).Where("id = ?", asset.ID).Count(&count)
	if count != 0 {
		t.Fatal("asset still present after delete")
	}
}

// ─── Assignee tests ───────────────────────────────────────────────────────────

func TestAssigneeCRUD(t *testing.T) {
	h, fApp := newTestApp(t)
	cookie := seedSession(t, h)

	// Create
	resp := postForm(t, fApp, "/assignees/create", url.Values{
		"full_name":    {"John Doe"},
		"email":        {"john@example.com"},
		"phone_number": {"08111222333"},
		"company":      {"PT Example"},
		"notes":        {"External vendor"},
	}, cookie)
	assertRedirect(t, resp)
	if strings.Contains(resp.Header.Get("Location"), "error") {
		t.Fatalf("unexpected error: %q", resp.Header.Get("Location"))
	}

	var assignee models.Assignee
	h.DB.First(&assignee)
	if assignee.FullName != "John Doe" || assignee.Email != "john@example.com" {
		t.Fatalf("assignee not persisted: %+v", assignee)
	}
	if assignee.AssigneeUUID == "" {
		t.Fatal("assignee UUID not generated")
	}

	// Update
	resp2 := postForm(t, fApp, "/assignees/update", url.Values{
		"id":           {strconv.FormatUint(uint64(assignee.ID), 10)},
		"full_name":    {"John Doe Jr."},
		"email":        {"john@example.com"},
		"phone_number": {"08111222444"},
		"company":      {"PT Updated"},
		"notes":        {""},
	}, cookie)
	assertRedirect(t, resp2)
	h.DB.First(&assignee, assignee.ID)
	if assignee.FullName != "John Doe Jr." {
		t.Fatalf("assignee name not updated: %q", assignee.FullName)
	}

	// Delete
	resp3 := postForm(t, fApp, "/assignees/delete",
		url.Values{"id": {strconv.FormatUint(uint64(assignee.ID), 10)}}, cookie)
	assertRedirect(t, resp3)
	var count int64
	h.DB.Model(&models.Assignee{}).Where("id = ?", assignee.ID).Count(&count)
	if count != 0 {
		t.Fatal("assignee still present after delete")
	}
}

func TestAssigneeCreateMissingName(t *testing.T) {
	h, fApp := newTestApp(t)
	cookie := seedSession(t, h)

	resp := postForm(t, fApp, "/assignees/create", url.Values{
		"full_name": {""},
		"email":     {"test@example.com"},
	}, cookie)
	assertRedirect(t, resp)
	if !strings.Contains(resp.Header.Get("Location"), "error") {
		t.Fatal("expected error redirect on missing full_name")
	}
}

// ─── Lending workflow tests ────────────────────────────────────────────────────

func TestLendAsset(t *testing.T) {
	h, fApp := newTestApp(t)
	cookie := seedSession(t, h)

	cat := seedCategory(t, h, "Devices")
	locID := uint(1)
	asset := seedAsset(t, h, "Laptop Pro", "movable", cat.ID, &locID)
	assignee := seedAssignee(t, h, "Jane Smith", "jane@example.com")

	resp := postForm(t, fApp, "/lending/lend", url.Values{
		"asset_id":    {strconv.FormatUint(uint64(asset.ID), 10)},
		"assignee_id": {strconv.FormatUint(uint64(assignee.ID), 10)},
		"notes":       {"Handle with care"},
	}, cookie)
	assertRedirect(t, resp)

	// LendingLog + HandoverForm should be created.
	var logCount int64
	h.DB.Model(&models.LendingLog{}).Where("asset_id = ? AND assignee_id = ?", asset.ID, assignee.ID).Count(&logCount)
	if logCount != 1 {
		t.Fatalf("expected 1 lending log, got %d", logCount)
	}
	var formCount int64
	h.DB.Model(&models.HandoverForm{}).Count(&formCount)
	if formCount != 1 {
		t.Fatal("expected 1 handover form to be created")
	}
}

func TestLendFixedAssetRejected(t *testing.T) {
	h, fApp := newTestApp(t)
	cookie := seedSession(t, h)

	cat := seedCategory(t, h, "Infrastructure")
	locID := uint(1)
	asset := seedAsset(t, h, "Server Rack", "fixed", cat.ID, &locID)
	assignee := seedAssignee(t, h, "Bob", "bob@example.com")

	resp := postForm(t, fApp, "/lending/lend", url.Values{
		"asset_id":    {strconv.FormatUint(uint64(asset.ID), 10)},
		"assignee_id": {strconv.FormatUint(uint64(assignee.ID), 10)},
	}, cookie)
	assertRedirect(t, resp)

	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "error") {
		t.Fatalf("expected error redirect when lending a fixed asset, got %q", loc)
	}

	// No lending log should be created.
	var count int64
	h.DB.Model(&models.LendingLog{}).Count(&count)
	if count != 0 {
		t.Fatal("lending log should not be created for a fixed asset")
	}
}

func TestReturnAssetWithCustomDate(t *testing.T) {
	h, fApp := newTestApp(t)
	cookie := seedSession(t, h)

	cat := seedCategory(t, h, "Devices")
	locID := uint(1)
	asset := seedAsset(t, h, "Tablet", "movable", cat.ID, &locID)
	assignee := seedAssignee(t, h, "Alice", "alice@example.com")
	llog, _ := seedLendingLog(t, h, asset.ID, assignee.ID)

	returnDate := "2026-05-15"
	resp := postForm(t, fApp, "/lending/return", url.Values{
		"lending_id":  {strconv.FormatUint(uint64(llog.ID), 10)},
		"returned_at": {returnDate},
	}, cookie)
	assertRedirect(t, resp)

	// Verify DB state.
	var updated models.LendingLog
	h.DB.First(&updated, llog.ID)
	if updated.Status != "returned" {
		t.Fatalf("expected status 'returned', got %q", updated.Status)
	}
	if updated.ReturnedAt == nil {
		t.Fatal("returned_at is nil after return")
	}
	if updated.ReturnedAt.Year() != 2026 || updated.ReturnedAt.Month() != 5 || updated.ReturnedAt.Day() != 15 {
		t.Fatalf("expected 2026-05-15 but got %v", updated.ReturnedAt)
	}
}

func TestReturnAssetDefaultsToToday(t *testing.T) {
	h, fApp := newTestApp(t)
	cookie := seedSession(t, h)

	cat := seedCategory(t, h, "Devices")
	locID := uint(1)
	asset := seedAsset(t, h, "Keyboard", "movable", cat.ID, &locID)
	assignee := seedAssignee(t, h, "Charlie", "charlie@example.com")
	llog, _ := seedLendingLog(t, h, asset.ID, assignee.ID)

	// POST without returned_at → should use today.
	before := time.Now()
	resp := postForm(t, fApp, "/lending/return", url.Values{
		"lending_id": {strconv.FormatUint(uint64(llog.ID), 10)},
	}, cookie)
	assertRedirect(t, resp)

	var updated models.LendingLog
	h.DB.First(&updated, llog.ID)
	if updated.Status != "returned" {
		t.Fatalf("expected status 'returned', got %q", updated.Status)
	}
	if updated.ReturnedAt == nil {
		t.Fatal("returned_at should be set")
	}
	// Should be within a few seconds of when we started.
	if updated.ReturnedAt.Before(before.Add(-5 * time.Second)) {
		t.Fatalf("returned_at (%v) is too far in the past", updated.ReturnedAt)
	}
}

func TestReturnAssetInvalidID(t *testing.T) {
	h, fApp := newTestApp(t)
	cookie := seedSession(t, h)

	cases := []url.Values{
		{"lending_id": {"0"}},
		{"lending_id": {"abc"}},
		{},
	}
	for _, v := range cases {
		resp := postForm(t, fApp, "/lending/return", v, cookie)
		assertRedirect(t, resp)
		if !strings.Contains(resp.Header.Get("Location"), "error") {
			t.Fatalf("expected error redirect for values %v, got %q", v, resp.Header.Get("Location"))
		}
	}
}

// ─── Handover form tests ───────────────────────────────────────────────────────

func TestHandoverSignGetValidToken(t *testing.T) {
	h, fApp := newTestApp(t)

	cat := seedCategory(t, h, "Devices")
	locID := uint(1)
	asset := seedAsset(t, h, "Monitor", "movable", cat.ID, &locID)
	assignee := seedAssignee(t, h, "Dave", "dave@example.com")
	_, form := seedLendingLog(t, h, asset.ID, assignee.ID)

	resp := getRequest(t, fApp, "/handover/sign?token="+form.FormToken, nil)
	// The public sign page renders without auth.
	assertStatus(t, resp, http.StatusOK)
}

func TestHandoverSignGetMissingToken(t *testing.T) {
	_, fApp := newTestApp(t)
	resp := getRequest(t, fApp, "/handover/sign", nil)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestHandoverSignGetInvalidToken(t *testing.T) {
	_, fApp := newTestApp(t)
	resp := getRequest(t, fApp, "/handover/sign?token=nonexistent-token", nil)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestHandoverSignGetAlreadySigned(t *testing.T) {
	h, fApp := newTestApp(t)

	cat := seedCategory(t, h, "Devices")
	locID := uint(1)
	asset := seedAsset(t, h, "Webcam", "movable", cat.ID, &locID)
	assignee := seedAssignee(t, h, "Eve", "eve@example.com")
	_, form := seedLendingLog(t, h, asset.ID, assignee.ID)

	// Mark as published (already signed).
	h.DB.Model(&form).Update("status", "published")

	resp := getRequest(t, fApp, "/handover/sign?token="+form.FormToken, nil)
	assertStatus(t, resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	// Should render the "already signed" page.
	if strings.Contains(string(body), "sign-canvas") {
		t.Fatal("should not show sign canvas for already-published form")
	}
}

func TestHandoverSignPostMissingSignature(t *testing.T) {
	h, fApp := newTestApp(t)

	cat := seedCategory(t, h, "Devices")
	locID := uint(1)
	asset := seedAsset(t, h, "Speaker", "movable", cat.ID, &locID)
	assignee := seedAssignee(t, h, "Frank", "frank@example.com")
	_, form := seedLendingLog(t, h, asset.ID, assignee.ID)

	resp := postForm(t, fApp, "/handover/sign", url.Values{
		"token":          {form.FormToken},
		"signature_data": {""},
	}, nil)
	// Missing signature → redirect back with error.
	assertRedirect(t, resp)
	if !strings.Contains(resp.Header.Get("Location"), "error") {
		t.Fatalf("expected error redirect, got %q", resp.Header.Get("Location"))
	}
}

func TestHandoverSignPostSuccess(t *testing.T) {
	h, fApp := newTestApp(t)

	cat := seedCategory(t, h, "Devices")
	locID := uint(1)
	asset := seedAsset(t, h, "Headphones", "movable", cat.ID, &locID)
	assignee := seedAssignee(t, h, "Grace", "grace@example.com")
	llog, form := seedLendingLog(t, h, asset.ID, assignee.ID)

	resp := postForm(t, fApp, "/handover/sign", url.Values{
		"token":          {form.FormToken},
		"signature_data": {minimalPNG},
	}, nil)
	// Should render success page (200).
	assertStatus(t, resp, http.StatusOK)

	// Verify DB updates.
	var updatedForm models.HandoverForm
	h.DB.First(&updatedForm, form.ID)
	if updatedForm.SignatureData != minimalPNG {
		t.Fatal("signature data not saved to DB")
	}
	if updatedForm.SignedAt == nil {
		t.Fatal("signed_at not set after signing")
	}
	// Status should be "signed" or "published" depending on PDF generation success.
	if updatedForm.Status != "signed" && updatedForm.Status != "published" {
		t.Fatalf("unexpected form status: %q", updatedForm.Status)
	}

	// LendingLog status should be updated to "active".
	var updatedLog models.LendingLog
	h.DB.First(&updatedLog, llog.ID)
	if updatedLog.Status != "active" {
		t.Fatalf("expected lending log status 'active', got %q", updatedLog.Status)
	}
}

func TestHandoverSignPostAlreadyPublished(t *testing.T) {
	h, fApp := newTestApp(t)

	cat := seedCategory(t, h, "Devices")
	locID := uint(1)
	asset := seedAsset(t, h, "Charger", "movable", cat.ID, &locID)
	assignee := seedAssignee(t, h, "Hank", "hank@example.com")
	_, form := seedLendingLog(t, h, asset.ID, assignee.ID)

	h.DB.Model(&form).Update("status", "published")

	resp := postForm(t, fApp, "/handover/sign", url.Values{
		"token":          {form.FormToken},
		"signature_data": {minimalPNG},
	}, nil)
	// Should redirect to the sign page (already signed).
	assertRedirect(t, resp)
}

// ─── Receipt download tests ────────────────────────────────────────────────────

func TestHandoverReceiptDownloadNotFound(t *testing.T) {
	h, fApp := newTestApp(t)
	cookie := seedSession(t, h)

	resp := getRequest(t, fApp, "/handover/receipt?form_uuid=nonexistent-uuid", cookie)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestHandoverReceiptDownloadNoReceiptYet(t *testing.T) {
	h, fApp := newTestApp(t)
	cookie := seedSession(t, h)

	cat := seedCategory(t, h, "Devices")
	locID := uint(1)
	asset := seedAsset(t, h, "Mouse", "movable", cat.ID, &locID)
	assignee := seedAssignee(t, h, "Ivy", "ivy@example.com")
	_, form := seedLendingLog(t, h, asset.ID, assignee.ID)
	// Form exists but has no receipt_path.

	resp := getRequest(t, fApp, "/handover/receipt?form_uuid="+form.FormUUID, cookie)
	assertStatus(t, resp, http.StatusNotFound)
}

// ─── Receipt utility tests ─────────────────────────────────────────────────────

func TestGenerateHandoverReceiptWithSignature(t *testing.T) {
	t.Cleanup(func() { os.RemoveAll("uploads/receipts") })

	data := utils.ReceiptData{
		AssetName:     "Test Laptop",
		AssetType:     "movable",
		SerialNumber:  "SN-TEST-001",
		Category:      "Electronics",
		AssigneeName:  "Test User",
		AssigneeEmail: "test@example.com",
		AssigneePhone: "08123456789",
		LentAt:        time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		SignedAt:      time.Now(),
		SignatureData: minimalPNG,
	}

	path, err := utils.GenerateHandoverReceipt(data, "test-uuid-001")
	if err != nil {
		t.Fatalf("GenerateHandoverReceipt returned error: %v", err)
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		t.Fatalf("PDF file not created at %q", path)
	}
	info, _ := os.Stat(path)
	if info.Size() == 0 {
		t.Fatal("generated PDF is empty")
	}
}

func TestGenerateHandoverReceiptWithoutSignature(t *testing.T) {
	t.Cleanup(func() { os.RemoveAll("uploads/receipts") })

	data := utils.ReceiptData{
		AssetName:     "Unsigned Asset",
		AssetType:     "movable",
		SerialNumber:  "SN-NOSIG-001",
		Category:      "General",
		AssigneeName:  "No Sig User",
		LentAt:        time.Now(),
		SignedAt:      time.Now(),
		SignatureData: "", // no signature
	}

	path, err := utils.GenerateHandoverReceipt(data, "test-uuid-nosig")
	if err != nil {
		t.Fatalf("GenerateHandoverReceipt without signature returned error: %v", err)
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		t.Fatalf("PDF file not created at %q", path)
	}
}

func TestGenerateHandoverReceiptInvalidBase64(t *testing.T) {
	t.Cleanup(func() { os.RemoveAll("uploads/receipts") })

	// Should fall back to placeholder box, not error out.
	data := utils.ReceiptData{
		AssetName:     "Bad Sig Asset",
		AssetType:     "movable",
		SerialNumber:  "SN-BADSIG-001",
		Category:      "General",
		AssigneeName:  "Bad Sig User",
		LentAt:        time.Now(),
		SignedAt:      time.Now(),
		SignatureData: "data:image/png;base64,!!!notvalidbase64!!!",
	}

	path, err := utils.GenerateHandoverReceipt(data, "test-uuid-badsig")
	if err != nil {
		t.Fatalf("GenerateHandoverReceipt with invalid base64 should not error: %v", err)
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		t.Fatalf("PDF file not created at %q", path)
	}
}

// ─── Forgot / reset password tests ────────────────────────────────────────────

func TestForgotPasswordGetRendersForm(t *testing.T) {
	_, fApp := newTestApp(t)
	resp := getRequest(t, fApp, "/forgot-password", nil)
	assertStatus(t, resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Reset Password") {
		t.Fatal("forgot-password page should contain 'Reset Password'")
	}
}

func TestForgotPasswordPostValidUser(t *testing.T) {
	h, fApp := newTestApp(t)

	// Create a user with a known email.
	h.DB.Create(&models.User{
		Username:   "resetme",
		Email:      "resetme@example.com",
		Password:   "$2a$10$placeholder",
		Active:     true,
		EmployeeID: "EMP-resetme",
	})

	resp := postForm(t, fApp, "/forgot-password", url.Values{
		"username": {"resetme"},
		"email":    {"resetme@example.com"},
	}, nil)
	assertStatus(t, resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Check Your Email") {
		t.Fatal("expected success confirmation page after valid reset request")
	}

	// A PasswordSetToken of kind "reset" should exist in DB.
	var tok models.PasswordSetToken
	h.DB.Where("user_id IN (SELECT id FROM users WHERE username = ?) AND kind = ?", "resetme", "reset").First(&tok)
	if tok.ID == 0 {
		t.Fatal("no reset token created in DB")
	}
	if tok.Token == "" {
		t.Fatal("reset token value is empty")
	}
}

func TestForgotPasswordPostWrongEmail(t *testing.T) {
	h, fApp := newTestApp(t)

	h.DB.Create(&models.User{
		Username:   "wrongemail",
		Email:      "correct@example.com",
		Password:   "$2a$10$placeholder",
		Active:     true,
		EmployeeID: "EMP-wrongemail",
	})

	resp := postForm(t, fApp, "/forgot-password", url.Values{
		"username": {"wrongemail"},
		"email":    {"wrong@example.com"},
	}, nil)
	// Should still show the "check your email" page (no enumeration).
	assertStatus(t, resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Check Your Email") {
		t.Fatal("mismatched email should show same success page to prevent enumeration")
	}

	// No token should be created.
	var count int64
	h.DB.Model(&models.PasswordSetToken{}).Count(&count)
	if count != 0 {
		t.Fatal("no token should be created for mismatched username/email")
	}
}

func TestForgotPasswordPostInactiveUser(t *testing.T) {
	h, fApp := newTestApp(t)

	h.DB.Create(&models.User{
		Username:   "inactive",
		Email:      "inactive@example.com",
		Password:   "$2a$10$placeholder",
		Active:     false,
		EmployeeID: "EMP-inactive", // inactive
	})

	resp := postForm(t, fApp, "/forgot-password", url.Values{
		"username": {"inactive"},
		"email":    {"inactive@example.com"},
	}, nil)
	assertStatus(t, resp, http.StatusOK)

	// No token for inactive accounts.
	var count int64
	h.DB.Model(&models.PasswordSetToken{}).Count(&count)
	if count != 0 {
		t.Fatal("no token should be created for inactive user")
	}
}

func TestForgotPasswordPostMissingFields(t *testing.T) {
	_, fApp := newTestApp(t)

	cases := []url.Values{
		{"username": {"someone"}},
		{"email": {"someone@example.com"}},
		{},
	}
	for _, v := range cases {
		resp := postForm(t, fApp, "/forgot-password", v, nil)
		assertStatus(t, resp, http.StatusOK)
		body, _ := io.ReadAll(resp.Body)
		// Should show the form with an error, not the success state.
		if strings.Contains(string(body), "Check Your Email") {
			t.Fatalf("empty fields should not show success page for %v", v)
		}
	}
}

func TestForgotPasswordDuplicateTokenInvalidated(t *testing.T) {
	h, fApp := newTestApp(t)

	h.DB.Create(&models.User{
		Username:   "dupuser",
		Email:      "dup@example.com",
		Password:   "$2a$10$placeholder",
		Active:     true,
		EmployeeID: "EMP-dupuser",
	})

	// Request a reset twice.
	for i := 0; i < 2; i++ {
		resp := postForm(t, fApp, "/forgot-password", url.Values{
			"username": {"dupuser"},
			"email":    {"dup@example.com"},
		}, nil)
		assertStatus(t, resp, http.StatusOK)
	}

	// Only one valid (unused) token should remain.
	var count int64
	h.DB.Model(&models.PasswordSetToken{}).
		Where("kind = ? AND used_at IS NULL", "reset").
		Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 active reset token after 2 requests, got %d", count)
	}
}

func TestLoginPageHasForgotPasswordLink(t *testing.T) {
	_, fApp := newTestApp(t)
	resp := getRequest(t, fApp, "/login", nil)
	assertStatus(t, resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "/forgot-password") {
		t.Fatal("login page should contain a link to /forgot-password")
	}
	if !strings.Contains(string(body), "/change-password") {
		t.Fatal("login page should contain a link to /change-password")
	}
}

// ─── Change Password tests (public — no session required) ──────────────────────

// seedUserWithPassword creates an active user with a bcrypt-hashed password and
// a known email, ready for the public change-password flow.
func seedUserWithPassword(t testing.TB, h *handlers.App, username, email, plainPassword string) models.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password for %s: %v", username, err)
	}
	u := models.User{
		Username:   username,
		Email:      email,
		Password:   string(hash),
		Active:     true,
		Role:       "viewer",
		EmployeeID: fmt.Sprintf("EMP-%s-%d", username, time.Now().UnixNano()),
	}
	if err := h.DB.Create(&u).Error; err != nil {
		t.Fatalf("seed user %s: %v", username, err)
	}
	return u
}

func TestChangePasswordGetRendersForm(t *testing.T) {
	_, fApp := newTestApp(t)

	// Public page — no cookie needed.
	resp := getRequest(t, fApp, "/change-password", nil)
	assertStatus(t, resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Change Password") {
		t.Fatal("change-password page should contain 'Change Password'")
	}
	// Form fields must be present.
	for _, field := range []string{"username", "email", "current_password", "new_password", "confirm_password"} {
		if !strings.Contains(string(body), `name="`+field+`"`) {
			t.Fatalf("change-password form is missing field %q", field)
		}
	}
}

func TestChangePasswordPostSuccess(t *testing.T) {
	h, fApp := newTestApp(t)
	seedUserWithPassword(t, h, "cpuser", "cpuser@example.com", "oldpass123")

	resp := postForm(t, fApp, "/change-password", url.Values{
		"username":         {"cpuser"},
		"email":            {"cpuser@example.com"},
		"current_password": {"oldpass123"},
		"new_password":     {"newpass456"},
		"confirm_password": {"newpass456"},
	}, nil) // no cookie
	assertStatus(t, resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Password Changed") {
		t.Fatal("expected success state after valid password change")
	}
	if !strings.Contains(string(body), "Go to Login") {
		t.Fatal("success state should contain 'Go to Login' link")
	}

	// New password must work; old one must not.
	var u models.User
	h.DB.Where("username = ?", "cpuser").First(&u)
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte("newpass456")) != nil {
		t.Fatal("new password not persisted correctly")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte("oldpass123")) == nil {
		t.Fatal("old password should no longer work after change")
	}
}

func TestChangePasswordPostWrongEmail(t *testing.T) {
	h, fApp := newTestApp(t)
	seedUserWithPassword(t, h, "wrongeml", "correct@example.com", "pass1234")

	resp := postForm(t, fApp, "/change-password", url.Values{
		"username":         {"wrongeml"},
		"email":            {"wrong@example.com"},
		"current_password": {"pass1234"},
		"new_password":     {"newpass456"},
		"confirm_password": {"newpass456"},
	}, nil)
	assertStatus(t, resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "incorrect") {
		t.Fatal("expected 'incorrect' error when email does not match")
	}
	if strings.Contains(string(body), "Password Changed") {
		t.Fatal("should not succeed with wrong email")
	}
}

func TestChangePasswordPostWrongUsername(t *testing.T) {
	h, fApp := newTestApp(t)
	seedUserWithPassword(t, h, "realuser", "real@example.com", "pass1234")

	resp := postForm(t, fApp, "/change-password", url.Values{
		"username":         {"ghostuser"},
		"email":            {"real@example.com"},
		"current_password": {"pass1234"},
		"new_password":     {"newpass456"},
		"confirm_password": {"newpass456"},
	}, nil)
	assertStatus(t, resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "incorrect") {
		t.Fatal("expected 'incorrect' error when username does not match")
	}
}

func TestChangePasswordPostWrongCurrentPassword(t *testing.T) {
	h, fApp := newTestApp(t)
	u := seedUserWithPassword(t, h, "pwcheck", "pwcheck@example.com", "correctpass")

	resp := postForm(t, fApp, "/change-password", url.Values{
		"username":         {"pwcheck"},
		"email":            {"pwcheck@example.com"},
		"current_password": {"wrongpass"},
		"new_password":     {"newpass456"},
		"confirm_password": {"newpass456"},
	}, nil)
	assertStatus(t, resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Current password is incorrect") {
		t.Fatal("expected 'Current password is incorrect' error")
	}

	// Password must be unchanged in DB.
	var unchanged models.User
	h.DB.First(&unchanged, u.ID)
	if bcrypt.CompareHashAndPassword([]byte(unchanged.Password), []byte("correctpass")) != nil {
		t.Fatal("password should not have changed after failed attempt")
	}
}

func TestChangePasswordPostMismatchedNewPasswords(t *testing.T) {
	h, fApp := newTestApp(t)
	seedUserWithPassword(t, h, "mismatch", "mismatch@example.com", "mypassword")

	resp := postForm(t, fApp, "/change-password", url.Values{
		"username":         {"mismatch"},
		"email":            {"mismatch@example.com"},
		"current_password": {"mypassword"},
		"new_password":     {"newpass123"},
		"confirm_password": {"differentpass"},
	}, nil)
	assertStatus(t, resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "do not match") {
		t.Fatal("expected 'do not match' error when confirm password differs")
	}
}

func TestChangePasswordPostTooShort(t *testing.T) {
	h, fApp := newTestApp(t)
	seedUserWithPassword(t, h, "tooshort", "tooshort@example.com", "mypassword")

	resp := postForm(t, fApp, "/change-password", url.Values{
		"username":         {"tooshort"},
		"email":            {"tooshort@example.com"},
		"current_password": {"mypassword"},
		"new_password":     {"short"},
		"confirm_password": {"short"},
	}, nil)
	assertStatus(t, resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "at least 8 characters") {
		t.Fatal("expected minimum-length error for short new password")
	}
}

func TestChangePasswordPostMissingFields(t *testing.T) {
	_, fApp := newTestApp(t)

	cases := []url.Values{
		{"username": {"u"}, "email": {"u@e.com"}, "current_password": {"p"}, "new_password": {"np"}},          // missing confirm
		{"username": {"u"}, "email": {"u@e.com"}, "current_password": {"p"}, "confirm_password": {"np"}},      // missing new
		{"username": {"u"}, "email": {"u@e.com"}, "new_password": {"np"}, "confirm_password": {"np"}},         // missing current
		{"username": {"u"}, "current_password": {"p"}, "new_password": {"np"}, "confirm_password": {"np"}},    // missing email
		{"email": {"u@e.com"}, "current_password": {"p"}, "new_password": {"np"}, "confirm_password": {"np"}}, // missing username
		{},
	}
	for _, v := range cases {
		resp := postForm(t, fApp, "/change-password", v, nil)
		assertStatus(t, resp, http.StatusOK)
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), "Password Changed") {
			t.Fatalf("should not succeed with missing fields: %v", v)
		}
		if !strings.Contains(string(body), "required") {
			t.Fatalf("expected 'required' error for missing fields %v", v)
		}
	}
}

func TestChangePasswordPostInactiveUser(t *testing.T) {
	h, fApp := newTestApp(t)

	hash, _ := bcrypt.GenerateFromPassword([]byte("pass1234"), bcrypt.DefaultCost)
	h.DB.Create(&models.User{
		Username:   "inactive",
		Email:      "inactive@example.com",
		Password:   string(hash),
		Active:     false,
		EmployeeID: "EMP-inactive",
	})

	resp := postForm(t, fApp, "/change-password", url.Values{
		"username":         {"inactive"},
		"email":            {"inactive@example.com"},
		"current_password": {"pass1234"},
		"new_password":     {"newpass456"},
		"confirm_password": {"newpass456"},
	}, nil)
	assertStatus(t, resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "Password Changed") {
		t.Fatal("inactive user should not be able to change password")
	}
	if !strings.Contains(string(body), "incorrect") {
		t.Fatal("expected 'incorrect' error for inactive user (same as unknown user)")
	}
}

func TestChangePasswordPreservesUsernameAndEmailOnError(t *testing.T) {
	h, fApp := newTestApp(t)
	seedUserWithPassword(t, h, "preserve", "preserve@example.com", "pass1234")

	resp := postForm(t, fApp, "/change-password", url.Values{
		"username":         {"preserve"},
		"email":            {"preserve@example.com"},
		"current_password": {"wrongpass"},
		"new_password":     {"newpass456"},
		"confirm_password": {"newpass456"},
	}, nil)
	assertStatus(t, resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	// username and email should be repopulated in the form value attributes.
	if !strings.Contains(string(body), "preserve") {
		t.Fatal("username should be preserved in form after error")
	}
	if !strings.Contains(string(body), "preserve@example.com") {
		t.Fatal("email should be preserved in form after error")
	}
}
