package main

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"
)

// sessionTTL is how long a session (pending or authenticated) remains valid.
const sessionTTL = 24 * time.Hour

func (a *App) loginGet(c fiber.Ctx) error {
	return c.Render("login", fiber.Map{"Title": "Login"})
}

func (a *App) loginPost(c fiber.Ctx) error {
	username := strings.TrimSpace(c.FormValue("username"))
	password := c.FormValue("password")

	if username != a.cfg.AdminUsername ||
		bcrypt.CompareHashAndPassword(a.adminHash, []byte(password)) != nil {
		return c.Render("login", fiber.Map{
			"Title": "Login",
			"Error": "invalid username or password",
		})
	}

	token := randomToken()
	expiry := time.Now().Add(sessionTTL)
	a.db.Create(&Session{Token: token, Username: username, Pending2FA: true, ExpiresAt: expiry})

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

func (a *App) login2FAGet(c fiber.Ctx) error {
	sess, ok := a.pendingSession(c)
	if !ok {
		return c.Redirect().To("/login")
	}
	return c.Render("login_2fa", fiber.Map{
		"Title":      "2FA Verification",
		"Pending2FA": true,
		"TotpSecret": a.cfg.TOTPSecret,
		"Username":   sess.Username,
	})
}

func (a *App) login2FAPost(c fiber.Ctx) error {
	sess, ok := a.pendingSession(c)
	if !ok {
		return c.Redirect().To("/login")
	}

	code := strings.TrimSpace(c.FormValue("code"))
	if !validateTOTP(a.cfg.TOTPSecret, code, time.Now()) {
		return c.Render("login_2fa", fiber.Map{
			"Title":      "2FA Verification",
			"Pending2FA": true,
			"TotpSecret": a.cfg.TOTPSecret,
			"Username":   sess.Username,
			"Error":      "invalid 2FA code",
		})
	}

	a.db.Model(sess).Updates(map[string]any{
		"authenticated": true,
		"pending_2fa":   false,
	})
	return c.Redirect().To("/categories")
}

func (a *App) logout(c fiber.Ctx) error {
	if token := c.Cookies("session_id"); token != "" {
		a.db.Where("token = ?", token).Delete(&Session{})
	}
	c.ClearCookie("session_id")
	return c.Redirect().To("/login")
}

// pendingSession returns the Session whose token is in the request cookie if it
// is in pending-2FA state (password verified, TOTP not yet checked).
func (a *App) pendingSession(c fiber.Ctx) (*Session, bool) {
	token := c.Cookies("session_id")
	if token == "" {
		return nil, false
	}
	var sess Session
	err := a.db.Where(
		"token = ? AND pending_2fa = ? AND expires_at > ?",
		token, true, time.Now(),
	).First(&sess).Error
	if err != nil {
		return nil, false
	}
	return &sess, true
}
