package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"

	"github.com/ABB-Broker/asset-management/internal/models"
	"github.com/ABB-Broker/asset-management/internal/totp"
)

// sessionTTL is how long a session (pending or authenticated) remains valid.
const sessionTTL = 24 * time.Hour

// LoginGet renders the login page.
func (a *App) LoginGet(c fiber.Ctx) error {
	return c.Render("login", fiber.Map{"Title": "Login"})
}

// LoginPost validates credentials, creates a pending-2FA session, and redirects.
func (a *App) LoginPost(c fiber.Ctx) error {
	username := strings.TrimSpace(c.FormValue("username"))
	password := c.FormValue("password")

	if username != a.Cfg.AdminUsername ||
		bcrypt.CompareHashAndPassword(a.AdminHash, []byte(password)) != nil {
		return c.Render("login", fiber.Map{
			"Title": "Login",
			"Error": "invalid username or password",
		})
	}

	token := models.RandomToken()
	expiry := time.Now().Add(sessionTTL)
	a.DB.Create(&models.Session{Token: token, Username: username, Pending2FA: true, ExpiresAt: expiry})

	c.Cookie(&fiber.Cookie{
		Name:     "session_id",
		Value:    token,
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Strict",
		Expires:  expiry,
	})
	return c.Redirect().To("/login/2fa")
}

// Login2FAGet renders the 2FA verification page for a pending session.
func (a *App) Login2FAGet(c fiber.Ctx) error {
	sess, ok := a.pendingSession(c)
	if !ok {
		return c.Redirect().To("/login")
	}
	return c.Render("login_2fa", fiber.Map{
		"Title":      "2FA Verification",
		"Pending2FA": true,
		"TotpSecret": a.Cfg.TOTPSecret,
		"Username":   sess.Username,
	})
}

// Login2FAPost validates the TOTP code and upgrades the session to fully authenticated.
func (a *App) Login2FAPost(c fiber.Ctx) error {
	sess, ok := a.pendingSession(c)
	if !ok {
		return c.Redirect().To("/login")
	}

	code := strings.TrimSpace(c.FormValue("code"))
	if !totp.Validate(a.Cfg.TOTPSecret, code, time.Now()) {
		return c.Render("login_2fa", fiber.Map{
			"Title":      "2FA Verification",
			"Pending2FA": true,
			"TotpSecret": a.Cfg.TOTPSecret,
			"Username":   sess.Username,
			"Error":      "invalid 2FA code",
		})
	}

	a.DB.Model(sess).Updates(map[string]any{
		"authenticated": true,
		"pending_2fa":   false,
	})
	return c.Redirect().To("/categories")
}

// Logout destroys the active session and redirects to the login page.
func (a *App) Logout(c fiber.Ctx) error {
	if token := c.Cookies("session_id"); token != "" {
		a.DB.Where("token = ?", token).Delete(&models.Session{})
	}
	c.ClearCookie("session_id")
	return c.Redirect().To("/login")
}

// pendingSession returns the Session whose token is in the request cookie if it
// is in pending-2FA state (password verified, TOTP not yet checked).
func (a *App) pendingSession(c fiber.Ctx) (*models.Session, bool) {
	token := c.Cookies("session_id")
	if token == "" {
		return nil, false
	}
	var sess models.Session
	err := a.DB.Where(
		"token = ? AND pending_2fa = ? AND expires_at > ?",
		token, true, time.Now(),
	).First(&sess).Error
	if err != nil {
		return nil, false
	}
	return &sess, true
}
