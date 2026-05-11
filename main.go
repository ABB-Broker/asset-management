package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Category struct {
	ID          int
	Name        string
	Description string
}

type Asset struct {
	ID           int
	Name         string
	CategoryID   int
	CategoryName string
	SerialNumber string
	PurchaseDate string
}

type Session struct {
	Username      string
	Authenticated bool
	Pending2FA    bool
}

type App struct {
	mu            sync.Mutex
	tmpl          *template.Template
	adminUsername string
	adminHash     []byte
	totpSecret    string
	sessions      map[string]Session
	categories    map[int]Category
	assets        map[int]Asset
	nextCategory  int
	nextAsset     int
}

type ViewData struct {
	Title       string
	CurrentPath string
	Error       string
	Message     string
	Username    string
	Pending2FA  bool
	TotpSecret  string
	Category    Category
	Categories  []Category
	Asset       Asset
	Assets      []Asset
}

func main() {
	adminUsername := envOrDefault("ADMIN_USERNAME", "admin")
	adminPassword := envOrDefault("ADMIN_PASSWORD", "admin123")
	totpSecret := envOrDefault("TOTP_SECRET", "JBSWY3DPEHPK3PXP")

	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash admin password: %v", err)
	}

	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("failed to parse templates: %v", err)
	}

	app := &App{
		tmpl:          tmpl,
		adminUsername: adminUsername,
		adminHash:     hash,
		totpSecret:    totpSecret,
		sessions:      make(map[string]Session),
		categories:    make(map[int]Category),
		assets:        make(map[int]Asset),
		nextCategory:  1,
		nextAsset:     1,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.redirectRoot)
	mux.HandleFunc("/login", app.handleLogin)
	mux.HandleFunc("/login/2fa", app.handleLogin2FA)
	mux.HandleFunc("/logout", app.handleLogout)
	mux.HandleFunc("/categories", app.authRequired(app.handleCategories))
	mux.HandleFunc("/categories/create", app.authRequired(app.handleCreateCategory))
	mux.HandleFunc("/categories/edit", app.authRequired(app.handleEditCategory))
	mux.HandleFunc("/categories/update", app.authRequired(app.handleUpdateCategory))
	mux.HandleFunc("/categories/delete", app.authRequired(app.handleDeleteCategory))
	mux.HandleFunc("/assets", app.authRequired(app.handleAssets))
	mux.HandleFunc("/assets/create", app.authRequired(app.handleCreateAsset))
	mux.HandleFunc("/assets/edit", app.authRequired(app.handleEditAsset))
	mux.HandleFunc("/assets/update", app.authRequired(app.handleUpdateAsset))
	mux.HandleFunc("/assets/delete", app.authRequired(app.handleDeleteAsset))

	log.Printf("Asset management running on http://localhost:8080 (admin: %s)", adminUsername)
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func (a *App) redirectRoot(w http.ResponseWriter, r *http.Request) {
	if s, ok := a.currentSession(r); ok && s.Authenticated {
		http.Redirect(w, r, "/categories", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.render(w, "login.html", ViewData{Title: "Login"})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if username != a.adminUsername || bcrypt.CompareHashAndPassword(a.adminHash, []byte(password)) != nil {
		a.render(w, "login.html", ViewData{Title: "Login", Error: "invalid username or password"})
		return
	}

	token := randomToken()
	a.mu.Lock()
	a.sessions[token] = Session{Username: username, Pending2FA: true}
	a.mu.Unlock()

	http.SetCookie(w, &http.Cookie{Name: "session_id", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/login/2fa", http.StatusSeeOther)
}

func (a *App) handleLogin2FA(w http.ResponseWriter, r *http.Request) {
	session, token, ok := a.pendingSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		a.render(w, "login_2fa.html", ViewData{Title: "2FA Verification", Pending2FA: true, TotpSecret: a.totpSecret, Username: session.Username})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	code := strings.TrimSpace(r.FormValue("code"))
	if !validateTOTP(a.totpSecret, code, time.Now()) {
		a.render(w, "login_2fa.html", ViewData{Title: "2FA Verification", Pending2FA: true, TotpSecret: a.totpSecret, Username: session.Username, Error: "invalid 2FA code"})
		return
	}

	a.mu.Lock()
	a.sessions[token] = Session{Username: session.Username, Authenticated: true}
	a.mu.Unlock()
	http.Redirect(w, r, "/categories", http.StatusSeeOther)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		a.mu.Lock()
		delete(a.sessions, cookie.Value)
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "session_id", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) authRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, ok := a.currentSession(r)
		if !ok || !s.Authenticated {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (a *App) handleCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	data := ViewData{
		Title:       "Category Master",
		CurrentPath: "/categories",
		Message:     q.Get("message"),
		Error:       q.Get("error"),
		Categories:  a.categoryList(),
	}
	a.render(w, "categories.html", data)
}

func (a *App) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	desc := strings.TrimSpace(r.FormValue("description"))
	if name == "" {
		http.Redirect(w, r, "/categories?error="+url.QueryEscape("category name is required"), http.StatusSeeOther)
		return
	}

	a.mu.Lock()
	id := a.nextCategory
	a.nextCategory++
	a.categories[id] = Category{ID: id, Name: name, Description: desc}
	a.mu.Unlock()
	http.Redirect(w, r, "/categories?message="+url.QueryEscape("category created"), http.StatusSeeOther)
}

func (a *App) handleEditCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Redirect(w, r, "/categories?error="+url.QueryEscape("invalid category id"), http.StatusSeeOther)
		return
	}

	a.mu.Lock()
	cat, ok := a.categories[id]
	list := a.categoryListLocked()
	a.mu.Unlock()
	if !ok {
		http.Redirect(w, r, "/categories?error="+url.QueryEscape("category not found"), http.StatusSeeOther)
		return
	}

	data := ViewData{Title: "Category Master", CurrentPath: "/categories", Category: cat, Categories: list}
	a.render(w, "categories.html", data)
}

func (a *App) handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		http.Redirect(w, r, "/categories?error="+url.QueryEscape("invalid category id"), http.StatusSeeOther)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	desc := strings.TrimSpace(r.FormValue("description"))
	if name == "" {
		http.Redirect(w, r, "/categories?error="+url.QueryEscape("category name is required"), http.StatusSeeOther)
		return
	}

	a.mu.Lock()
	cat, ok := a.categories[id]
	if ok {
		cat.Name = name
		cat.Description = desc
		a.categories[id] = cat
		for k, asset := range a.assets {
			if asset.CategoryID == id {
				asset.CategoryName = name
				a.assets[k] = asset
			}
		}
	}
	a.mu.Unlock()
	if !ok {
		http.Redirect(w, r, "/categories?error="+url.QueryEscape("category not found"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/categories?message="+url.QueryEscape("category updated"), http.StatusSeeOther)
}

func (a *App) handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		http.Redirect(w, r, "/categories?error="+url.QueryEscape("invalid category id"), http.StatusSeeOther)
		return
	}

	a.mu.Lock()
	_, ok := a.categories[id]
	if ok {
		delete(a.categories, id)
		for k, asset := range a.assets {
			if asset.CategoryID == id {
				delete(a.assets, k)
			}
		}
	}
	a.mu.Unlock()
	if !ok {
		http.Redirect(w, r, "/categories?error="+url.QueryEscape("category not found"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/categories?message="+url.QueryEscape("category deleted"), http.StatusSeeOther)
}

func (a *App) handleAssets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	data := ViewData{
		Title:       "Asset Master",
		CurrentPath: "/assets",
		Message:     q.Get("message"),
		Error:       q.Get("error"),
		Assets:      a.assetList(),
		Categories:  a.categoryList(),
	}
	a.render(w, "assets.html", data)
}

func (a *App) handleCreateAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	asset, err := a.assetFromRequest(r)
	if err != nil {
		http.Redirect(w, r, "/assets?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	a.mu.Lock()
	asset.ID = a.nextAsset
	a.nextAsset++
	a.assets[asset.ID] = asset
	a.mu.Unlock()
	http.Redirect(w, r, "/assets?message="+url.QueryEscape("asset created"), http.StatusSeeOther)
}

func (a *App) handleEditAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Redirect(w, r, "/assets?error="+url.QueryEscape("invalid asset id"), http.StatusSeeOther)
		return
	}

	a.mu.Lock()
	asset, ok := a.assets[id]
	a.mu.Unlock()
	if !ok {
		http.Redirect(w, r, "/assets?error="+url.QueryEscape("asset not found"), http.StatusSeeOther)
		return
	}
	q := r.URL.Query()
	data := ViewData{
		Title:       "Asset Master",
		CurrentPath: "/assets",
		Message:     q.Get("message"),
		Error:       q.Get("error"),
		Assets:      a.assetList(),
		Categories:  a.categoryList(),
		Asset:       asset,
	}
	a.render(w, "assets.html", data)
}

func (a *App) handleUpdateAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		http.Redirect(w, r, "/assets?error="+url.QueryEscape("invalid asset id"), http.StatusSeeOther)
		return
	}
	asset, err := a.assetFromRequest(r)
	if err != nil {
		http.Redirect(w, r, "/assets?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	a.mu.Lock()
	_, ok := a.assets[id]
	if ok {
		asset.ID = id
		a.assets[id] = asset
	}
	a.mu.Unlock()
	if !ok {
		http.Redirect(w, r, "/assets?error="+url.QueryEscape("asset not found"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/assets?message="+url.QueryEscape("asset updated"), http.StatusSeeOther)
}

func (a *App) handleDeleteAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		http.Redirect(w, r, "/assets?error="+url.QueryEscape("invalid asset id"), http.StatusSeeOther)
		return
	}

	a.mu.Lock()
	_, ok := a.assets[id]
	if ok {
		delete(a.assets, id)
	}
	a.mu.Unlock()
	if !ok {
		http.Redirect(w, r, "/assets?error="+url.QueryEscape("asset not found"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/assets?message="+url.QueryEscape("asset deleted"), http.StatusSeeOther)
}

func (a *App) assetFromRequest(r *http.Request) (Asset, error) {
	name := strings.TrimSpace(r.FormValue("name"))
	serial := strings.TrimSpace(r.FormValue("serial_number"))
	purchaseDate := strings.TrimSpace(r.FormValue("purchase_date"))
	categoryID, err := strconv.Atoi(r.FormValue("category_id"))
	if err != nil {
		return Asset{}, fmt.Errorf("invalid category id")
	}
	if name == "" || serial == "" || purchaseDate == "" {
		return Asset{}, fmt.Errorf("all asset fields are required")
	}

	a.mu.Lock()
	cat, ok := a.categories[categoryID]
	a.mu.Unlock()
	if !ok {
		return Asset{}, fmt.Errorf("category not found")
	}

	return Asset{Name: name, CategoryID: categoryID, CategoryName: cat.Name, SerialNumber: serial, PurchaseDate: purchaseDate}, nil
}

func (a *App) currentSession(r *http.Request) (Session, bool) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return Session{}, false
	}
	a.mu.Lock()
	s, ok := a.sessions[cookie.Value]
	a.mu.Unlock()
	return s, ok
}

func (a *App) pendingSession(r *http.Request) (Session, string, bool) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return Session{}, "", false
	}
	a.mu.Lock()
	s, ok := a.sessions[cookie.Value]
	a.mu.Unlock()
	if !ok || !s.Pending2FA {
		return Session{}, "", false
	}
	return s, cookie.Value, true
}

func (a *App) categoryList() []Category {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.categoryListLocked()
}

func (a *App) categoryListLocked() []Category {
	list := make([]Category, 0, len(a.categories))
	for _, c := range a.categories {
		list = append(list, c)
	}
	for i := 0; i < len(list)-1; i++ {
		for j := i + 1; j < len(list); j++ {
			if list[i].ID > list[j].ID {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	return list
}

func (a *App) assetList() []Asset {
	a.mu.Lock()
	defer a.mu.Unlock()
	list := make([]Asset, 0, len(a.assets))
	for _, asset := range a.assets {
		list = append(list, asset)
	}
	for i := 0; i < len(list)-1; i++ {
		for j := i + 1; j < len(list); j++ {
			if list[i].ID > list[j].ID {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	return list
}

func (a *App) render(w http.ResponseWriter, page string, data ViewData) {
	if err := a.tmpl.ExecuteTemplate(w, page, data); err != nil {
		http.Error(w, "template rendering error", http.StatusInternalServerError)
	}
}

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

func randomToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
