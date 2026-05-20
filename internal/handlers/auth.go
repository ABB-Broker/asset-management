package handlers

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"

	"github.com/ABB-Broker/asset-management/internal/models"
)

// sessionTTL is how long a session (pending or authenticated) remains valid.
const sessionTTL = 24 * time.Hour

// otpTTL is how long an emailed OTP code is valid.
const otpTTL = 10 * time.Minute

// LoginGet renders the login page.
func (a *App) LoginGet(c fiber.Ctx) error {
	return c.Render("login", fiber.Map{"Title": "Login"})
}

// LoginPost validates credentials, sends a 2FA OTP via email, and redirects.
func (a *App) LoginPost(c fiber.Ctx) error {
	username := strings.TrimSpace(c.FormValue("username"))
	password := c.FormValue("password")

	authenticated := false
	var userEmail string

	// Check built-in admin account.
	if username == a.Cfg.AdminUsername && bcrypt.CompareHashAndPassword(a.AdminHash, []byte(password)) == nil {
		authenticated = true
		// Admin has no user record — use a configured or empty email for OTP.
		// If you want admin 2FA via email, set ADMIN_EMAIL in config.
	}

	if !authenticated {
		var u models.User
		if err := a.DB.Where("username = ? AND active = ?", username, true).First(&u).Error; err == nil {
			if u.Password != "" && bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) == nil {
				authenticated = true
				userEmail = u.Email
			}
		}
	}

	if !authenticated {
		return c.Render("login", fiber.Map{
			"Title": "Login",
			"Error": "invalid username or password",
		})
	}

	// Create pending session.
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

	// Generate and store OTP code.
	code := generateOTPCode()
	a.DB.Create(&models.EmailOTP{
		Code:      code,
		Username:  username,
		ExpiresAt: time.Now().Add(otpTTL),
	})

	// Send OTP email (best-effort; fall back to dev bypass on failure).
	if userEmail != "" {
		_ = a.sendOTPEmail(userEmail, username, code)
	}

	return c.Redirect().To("/login/2fa")
}

// Login2FAGet renders the 2FA verification page for a pending session.
func (a *App) Login2FAGet(c fiber.Ctx) error {
	sess, ok := a.pendingSession(c)
	if !ok {
		return c.Redirect().To("/login")
	}
	showBypass := a.Cfg.DevOTPBypass != ""
	return c.Render("login_2fa", fiber.Map{
		"Title":      "2FA Verification",
		"Pending2FA": true,
		"ShowBypass": showBypass,
		"DevBypass":  a.Cfg.DevOTPBypass,
		"Username":   sess.Username,
	})
}

// Login2FAPost validates the emailed OTP code and upgrades the session.
func (a *App) Login2FAPost(c fiber.Ctx) error {
	sess, ok := a.pendingSession(c)
	if !ok {
		return c.Redirect().To("/login")
	}

	code := strings.TrimSpace(c.FormValue("code"))

	// Dev bypass.
	bypassOK := a.Cfg.DevOTPBypass != "" && code == a.Cfg.DevOTPBypass

	if !bypassOK {
		var otp models.EmailOTP
		err := a.DB.Where(
			"username = ? AND code = ? AND used_at IS NULL AND expires_at > ?",
			sess.Username, code, time.Now(),
		).First(&otp).Error
		if err != nil {
			return c.Render("login_2fa", fiber.Map{
				"Title":      "2FA Verification",
				"Pending2FA": true,
				"ShowBypass": a.Cfg.DevOTPBypass != "",
				"DevBypass":  a.Cfg.DevOTPBypass,
				"Username":   sess.Username,
				"Error":      "invalid or expired OTP code",
			})
		}
		// Mark OTP as used.
		now := time.Now()
		a.DB.Model(&otp).Update("used_at", &now)
	}

	a.DB.Model(sess).Updates(map[string]any{
		"authenticated": true,
		"pending_2fa":   false,
	})
	return c.Redirect().To("/locations")
}

// Logout destroys the active session and redirects to the login page.
func (a *App) Logout(c fiber.Ctx) error {
	if token := c.Cookies("session_id"); token != "" {
		a.DB.Where("token = ?", token).Delete(&models.Session{})
	}
	c.ClearCookie("session_id")
	return c.Redirect().To("/login")
}

// pendingSession returns the Session in pending-2FA state from the cookie.
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

// ─── Forgot / Reset Password (public) ────────────────────────────────────────
// ForgotPasswordGet renders the forgot-password form.
// GET /forgot-password
func (a *App) ForgotPasswordGet(c fiber.Ctx) error {
	return c.Render("forgot_password", fiber.Map{"Title": "Reset Password"})
}

// ForgotPasswordPost validates the submitted username + email pair.
// If they match an active user account, a reset token is created and emailed.
// The response always shows the same "check your email" confirmation to avoid
// leaking whether a particular username or email is registered.
// POST /forgot-password
func (a *App) ForgotPasswordPost(c fiber.Ctx) error {
	username := strings.TrimSpace(c.FormValue("username"))
	email := strings.TrimSpace(c.FormValue("email"))
	sent := fiber.Map{"Title": "Reset Password", "Sent": true}
	if username == "" || email == "" {
		return c.Render("forgot_password", fiber.Map{"Title": "Reset Password", "Error": "Please fill in both fields.", "Username": username, "Email": email})
	}

	// Look up the user. We proceed silently on any mismatch to prevent
	// username/email enumeration.
	var u models.User
	err := a.DB.Where("username = ? AND email = ? AND active = ?", username, email, true).First(&u).Error
	if err != nil {
		// No match found — show the same success message to prevent enumeration.
		return c.Render("forgot_password", sent)
	}

	// Invalidate any existing unused reset tokens for this user.
	a.DB.Where("user_id = ? AND kind = ? AND used_at IS NULL", u.ID, "reset").Delete(&models.PasswordSetToken{})

	tok := models.PasswordSetToken{UserID: u.ID, Kind: "reset"}

	if err := a.DB.Create(&tok).Error; err != nil {
		return c.Render("forgot_password", sent)
	}

	link := fmt.Sprintf("%s/set-password?token=%s", a.Cfg.BaseURL, tok.Token)
	_ = a.sendSetPasswordEmail(u.Email, u.FullName, link, "reset")

	return c.Render("forgot_password", sent)
}

// ─── Change Password (public — no session required) ───────────────────────────
// Any user can change their password by providing their username, registered
// email address, and current password. No active session is needed.

// ChangePasswordGet renders the standalone change-password page.
// GET /change-password
func (a *App) ChangePasswordGet(c fiber.Ctx) error {
	return c.Render("change_password", fiber.Map{"Title": "Change Password"})
}

// ChangePasswordPost processes the change-password form.
// Validation order matches what the template communicates to the user:
//  1. All fields present
//  2. username + email match an active account
//  3. current_password is correct
//  4. new_password meets the minimum length
//  5. new_password == confirm_password
//
// On success the same template is re-rendered with Success: true, which
// switches to the confirmation view with a "Go to Login" button.
// POST /change-password
func (a *App) ChangePasswordPost(c fiber.Ctx) error {
	username := strings.TrimSpace(c.FormValue("username"))
	email := strings.TrimSpace(c.FormValue("email"))
	currentPw := c.FormValue("current_password")
	newPw := c.FormValue("new_password")
	confirmPw := c.FormValue("confirm_password")

	// Helper: re-render the form with an error, preserving username/email so
	// the user doesn't have to retype them.
	renderErr := func(msg string) error {
		return c.Render("change_password", fiber.Map{
			"Title":    "Change Password",
			"Error":    msg,
			"Username": username,
			"Email":    email,
		})
	}

	// 1. All fields must be present.
	if username == "" || email == "" || currentPw == "" || newPw == "" || confirmPw == "" {
		return renderErr("All fields are required.")
	}

	// 2. Look up a matching active user. Use a deliberately vague error message
	//    to avoid leaking whether the username or email is registered.
	var u models.User
	if err := a.DB.Where("username = ? AND email = ? AND active = ?", username, email, true).First(&u).Error; err != nil {
		return renderErr("Username or email address is incorrect.")
	}

	// 3. Verify current password.
	if u.Password == "" || bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(currentPw)) != nil {
		return renderErr("Current password is incorrect.")
	}

	// 4. Enforce minimum length (mirrors the HTML minlength="8").
	if len(newPw) < 8 {
		return renderErr("New password must be at least 8 characters.")
	}

	// 5. Confirm passwords must match.
	if newPw != confirmPw {
		return renderErr("New passwords do not match.")
	}

	// Hash and persist the new password.
	hash, err := bcrypt.GenerateFromPassword([]byte(newPw), bcrypt.DefaultCost)
	if err != nil {
		return renderErr("Failed to update password. Please try again.")
	}
	if err := a.DB.Model(&u).Update("password", string(hash)).Error; err != nil {
		return renderErr("Failed to save new password. Please try again.")
	}

	return c.Render("change_password", fiber.Map{
		"Title":   "Change Password",
		"Success": true,
	})
}

// ─── OTP helpers ─────────────────────────────────────────────────────────────
func generateOTPCode() string {
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}
