package handlers

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"

	"github.com/ABB-Broker/asset-management/internal/models"
)

// UsersIndex renders the User Master list with an inline create form.
func (a *App) UsersIndex(c fiber.Ctx) error {
	var users []models.User
	a.DB.Order("id asc").Find(&users)
	return c.Render("users", fiber.Map{
		"Title":       "User Master",
		"CurrentPath": "/users",
		"Message":     c.Query("message"),
		"Error":       c.Query("error"),
		"Users":       users,
		"User":        models.User{},
	})
}

// UsersCreate persists a new user account and its linked Assignee row.
func (a *App) UsersCreate(c fiber.Ctx) error {
	u, err := a.userFromCtx(c)
	if err != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape(err.Error()))
	}

	password := c.FormValue("password")
	if password == "" {
		return c.Redirect().To("/users?error=" + url.QueryEscape("password is required"))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape("failed to hash password"))
	}
	u.Password = string(hash)

	// Use a transaction so User + Assignee are created atomically.
	tx := a.DB.Begin()

	if res := tx.Create(&u); res.Error != nil {
		tx.Rollback()
		return c.Redirect().To("/users?error=" + url.QueryEscape("username or email already exists"))
	}

	// Auto-create the linked Assignee row for internal employees.
	assignee := models.Assignee{
		FullName:    u.FullName,
		Email:       u.Email,
		PhoneNumber: u.PhoneNumber,
		Department:  u.Department,
		Position:    u.Position,
		EmployeeID:  u.EmployeeID,
		UserID:      &u.ID,
	}
	if res := tx.Create(&assignee); res.Error != nil {
		tx.Rollback()
		return c.Redirect().To("/users?error=" + url.QueryEscape("failed to create assignee record"))
	}

	// Link the user back to the assignee.
	tx.Model(&u).Update("assignee_id", assignee.ID)

	tx.Commit()

	return c.Redirect().To("/users?message=" + url.QueryEscape("user created"))
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
	a.DB.Order("id asc").Find(&users)
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

	// Sync the linked Assignee if it exists.
	if existing.AssigneeID != nil {
		tx.Model(&models.Assignee{}).Where("id = ?", *existing.AssigneeID).Updates(map[string]any{
			"full_name":    updated.FullName,
			"email":        updated.Email,
			"phone_number": updated.PhoneNumber,
			"department":   updated.Department,
			"position":     updated.Position,
			"employee_id":  updated.EmployeeID,
		})
	}

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

	// Delete the linked internal Assignee first (if one exists).
	if u.AssigneeID != nil {
		if err := tx.Delete(&models.Assignee{}, *u.AssigneeID).Error; err != nil {
			tx.Rollback()
			return c.Redirect().To("/users?error=" + url.QueryEscape("failed to remove linked assignee"))
		}
	}

	// Now delete the user.
	if err := tx.Delete(&u).Error; err != nil {
		tx.Rollback()
		return c.Redirect().To("/users?error=" + url.QueryEscape("failed to delete user"))
	}

	tx.Commit()

	return c.Redirect().To("/users?message=" + url.QueryEscape("user deleted"))
}

// validRoles is the set of accepted role values for User accounts.
var validRoles = map[string]bool{"admin": true, "editor": true, "viewer": true}

// userFromCtx parses and validates user fields from a Fiber form context.
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
