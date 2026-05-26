package handlers

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"

	"github.com/ABB-Broker/asset-management/internal/models"
)

// UsersIndex renders the User Master list with an inline create form.
func (a *App) UsersIndex(c fiber.Ctx) error {
	var users []models.User
	a.DB.Order("user_no asc").Find(&users)
	return c.Render("users", fiber.Map{
		"Title":       "User Master",
		"CurrentPath": "/users",
		"Message":     c.Query("message"),
		"Error":       c.Query("error"),
		"Users":       users,
		"User":        models.User{},
	})
}

// UsersCreate creates a new user without a password and sends an invite email.
// POST /users/create
func (a *App) UsersCreate(c fiber.Ctx) error {
	u, err := a.userFromCtx(c)
	if err != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape(err.Error()))
	}

	tx := a.DB.Begin()
	if res := tx.Create(&u); res.Error != nil {
		tx.Rollback()
		return c.Redirect().To("/users?error=" + url.QueryEscape("username or email already exists"))
	}

	// Create the linked assignee record (UserNo points back to the user).
	// This is the ONLY link between users and assignees — no reverse FK on users.
	assignee := models.Assignee{
		FullName:    u.FullName,
		Email:       u.Email,
		PhoneNumber: u.PhoneNumber,
		Department:  u.Department,
		Position:    u.Position,
		EmployeeID:  u.EmployeeID,
		UserNo:      &u.UserNo,
	}
	if res := tx.Create(&assignee); res.Error != nil {
		tx.Rollback()
		return c.Redirect().To("/users?error=" + url.QueryEscape("failed to create assignee record"))
	}
	tx.Commit()

	// Generate invite token and send email (best-effort).
	if u.Email != "" {
		tok := models.PasswordSetToken{UserNo: u.UserNo, Kind: "invite"}
		if err := a.DB.Create(&tok).Error; err == nil {
			link := fmt.Sprintf("%s/set-password?token=%s", a.Cfg.BaseURL, tok.Token)
			_ = a.sendSetPasswordEmail(u.Email, u.FullName, link, "invite")
		}
	}

	return c.Redirect().To("/users?message=" + url.QueryEscape("user created — invite email sent"))
}

// UsersEdit renders the edit form pre-filled with the user's current data.
func (a *App) UsersEdit(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Query("id"), 10, 64)
	if err != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape("invalid user id"))
	}
	var u models.User
	if err := a.DB.First(&u, id).Error; err != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape("user not found"))
	}
	var users []models.User
	a.DB.Order("user_no asc").Find(&users)
	return c.Render("users", fiber.Map{
		"Title":       "User Master",
		"CurrentPath": "/users",
		"User":        u,
		"Users":       users,
	})
}

// UsersUpdate persists changes to an existing user account and syncs the linked Assignee.
func (a *App) UsersUpdate(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, 64)
	if err != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape("invalid user id"))
	}
	var existing models.User
	if err := a.DB.First(&existing, id).Error; err != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape("user not found"))
	}
	updated, err := a.userFromCtx(c)
	if err != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape(err.Error()))
	}

	tx := a.DB.Begin()
	if res := tx.Model(&existing).Updates(map[string]any{
		"username":     updated.Username,
		"email":        updated.Email,
		"full_name":    updated.FullName,
		"phone_number": updated.PhoneNumber,
		"department":   updated.Department,
		"position":     updated.Position,
		"employee_id":  updated.EmployeeID,
		"role":         updated.Role,
		"active":       updated.Active,
	}); res.Error != nil {
		tx.Rollback()
		return c.Redirect().To("/users?error=" + url.QueryEscape("username or email already in use"))
	}

	// Sync the linked assignee record.
	// We look it up by user_no — there is no AssigneeNo on the User struct anymore.
	tx.Model(&models.Assignee{}).Where("user_no = ?", existing.UserNo).Updates(map[string]any{
		"full_name":    updated.FullName,
		"email":        updated.Email,
		"phone_number": updated.PhoneNumber,
		"department":   updated.Department,
		"position":     updated.Position,
		"employee_id":  updated.EmployeeID,
	})

	tx.Commit()
	return c.Redirect().To("/users?message=" + url.QueryEscape("user updated"))
}

// UsersDelete removes a user account and its linked internal Assignee record.
func (a *App) UsersDelete(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, 64)
	if err != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape("invalid user id"))
	}
	var u models.User
	if err := a.DB.First(&u, id).Error; err != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape("user not found"))
	}

	tx := a.DB.Begin()

	// Delete the linked assignee by user_no — no AssigneeNo on User anymore.
	if err := tx.Where("user_no = ?", u.UserNo).Delete(&models.Assignee{}).Error; err != nil {
		tx.Rollback()
		return c.Redirect().To("/users?error=" + url.QueryEscape("failed to remove linked assignee"))
	}

	if err := tx.Delete(&u).Error; err != nil {
		tx.Rollback()
		return c.Redirect().To("/users?error=" + url.QueryEscape("failed to delete user"))
	}
	tx.Commit()
	return c.Redirect().To("/users?message=" + url.QueryEscape("user deleted"))
}

// UsersResendInvite generates a fresh invite token and resends the email.
// POST /users/resend-invite
func (a *App) UsersResendInvite(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, 64)
	if err != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape("invalid user id"))
	}
	var u models.User
	if err := a.DB.First(&u, id).Error; err != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape("user not found"))
	}
	if u.Email == "" {
		return c.Redirect().To("/users?error=" + url.QueryEscape("user has no email address"))
	}
	tok := models.PasswordSetToken{UserNo: u.UserNo, Kind: "invite"}
	if err := a.DB.Create(&tok).Error; err != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape("failed to create invite token"))
	}
	link := fmt.Sprintf("%s/set-password?token=%s", a.Cfg.BaseURL, tok.Token)
	_ = a.sendSetPasswordEmail(u.Email, u.FullName, link, "invite")
	return c.Redirect().To("/users?message=" + url.QueryEscape("invite email resent to "+u.Email))
}

// ─── Set Password (invite & forgot-password flow, no auth required) ──────────

// SetPasswordGet renders the set-password form.
// GET /set-password?token=...
func (a *App) SetPasswordGet(c fiber.Ctx) error {
	token := c.Query("token")
	if token == "" {
		return c.Render("set_password", fiber.Map{"Title": "Set Password", "Error": "Missing token."})
	}
	var tok models.PasswordSetToken
	if err := a.DB.Preload("User").Where("token = ?", token).First(&tok).Error; err != nil {
		return c.Render("set_password", fiber.Map{"Title": "Set Password", "Error": "This link is invalid."})
	}
	if !tok.IsValid() {
		return c.Render("set_password", fiber.Map{"Title": "Set Password", "Error": "This link has already been used or has expired."})
	}
	title := "Set Your Password"
	if tok.Kind == "reset" {
		title = "Change Your Password"
	}
	return c.Render("set_password", fiber.Map{
		"Title": title,
		"Token": token,
		"Kind":  tok.Kind,
		"User":  tok.User,
	})
}

// SetPasswordPost processes the new password from the invite / reset link.
// POST /set-password
func (a *App) SetPasswordPost(c fiber.Ctx) error {
	token := strings.TrimSpace(c.FormValue("token"))
	password := c.FormValue("password")
	confirm := c.FormValue("confirm_password")

	renderErr := func(msg string) error {
		return c.Render("set_password", fiber.Map{"Title": "Set Password", "Token": token, "Error": msg})
	}

	if token == "" {
		return renderErr("Invalid token.")
	}
	var tok models.PasswordSetToken
	if err := a.DB.Preload("User").Where("token = ?", token).First(&tok).Error; err != nil {
		return renderErr("This link is invalid.")
	}
	if !tok.IsValid() {
		return renderErr("This link has already been used or has expired.")
	}
	if len(password) < 8 {
		return renderErr("Password must be at least 8 characters.")
	}
	if password != confirm {
		return renderErr("Passwords do not match.")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return renderErr("Failed to hash password. Please try again.")
	}
	now := time.Now()
	tx := a.DB.Begin()
	tx.Model(&tok.User).Update("password", string(hash))
	tx.Model(&tok).Update("used_at", &now)
	tx.Commit()

	return c.Render("set_password", fiber.Map{
		"Title":   "Password Set",
		"Success": true,
		"Kind":    tok.Kind,
	})
}

// ─── Validation helpers ───────────────────────────────────────────────────────

var validRoles = map[string]bool{"admin": true, "editor": true, "viewer": true}

func (a *App) userFromCtx(c fiber.Ctx) (models.User, error) {
	username := strings.TrimSpace(c.FormValue("username"))
	email := strings.TrimSpace(c.FormValue("email"))
	fullName := strings.TrimSpace(c.FormValue("full_name"))
	phoneNumber := strings.TrimSpace(c.FormValue("phone_number"))
	department := strings.TrimSpace(c.FormValue("department"))
	position := strings.TrimSpace(c.FormValue("position"))
	employeeID := strings.TrimSpace(c.FormValue("employee_id"))
	role := strings.TrimSpace(c.FormValue("role"))
	active := c.FormValue("active") != "false"

	if username == "" {
		return models.User{}, fiber.NewError(fiber.StatusBadRequest, "username is required")
	}
	if role == "" {
		role = "viewer"
	}
	if !validRoles[role] {
		return models.User{}, fiber.NewError(fiber.StatusBadRequest, "role must be admin, editor, or viewer")
	}
	return models.User{
		Username:    username,
		Email:       email,
		FullName:    fullName,
		PhoneNumber: phoneNumber,
		Department:  department,
		Position:    position,
		EmployeeID:  employeeID,
		Role:        role,
		Active:      active,
	}, nil
}
