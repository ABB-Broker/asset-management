package handlers

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/ABB-Broker/asset-management/internal/models"
)

// UsersIndex renders the User Master list with an inline create form.
//
// @Summary     List users
// @Description Returns all user accounts.
// @Tags        users
// @Produce     json
// @Success     200 {array} models.User
// @Security    SessionCookie
// @Router      /api/v1/users [get]
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

// UsersCreate persists a new user account.
//
// @Summary     Create user
// @Description Creates a new user account.
// @Tags        users
// @Accept      application/x-www-form-urlencoded
// @Produce     json
// @Param       username  formData string true  "Username"
// @Param       email     formData string false "Email address"
// @Param       full_name formData string false "Full name"
// @Param       role      formData string true  "Role (admin|editor|viewer)"
// @Param       active    formData string false "Active (true|false)"
// @Success     303
// @Security    SessionCookie
// @Router      /api/v1/users [post]
func (a *App) UsersCreate(c fiber.Ctx) error {
	u, err := a.userFromCtx(c)
	if err != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape(err.Error()))
	}
	if res := a.DB.Create(&u); res.Error != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape("username or email already exists"))
	}
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

// UsersUpdate persists changes to an existing user account.
//
// @Summary     Update user
// @Description Updates an existing user account.
// @Tags        users
// @Accept      application/x-www-form-urlencoded
// @Produce     json
// @Param       id        formData int    true  "User ID"
// @Param       username  formData string true  "Username"
// @Param       email     formData string false "Email address"
// @Param       full_name formData string false "Full name"
// @Param       role      formData string true  "Role (admin|editor|viewer)"
// @Param       active    formData string false "Active (true|false)"
// @Success     303
// @Security    SessionCookie
// @Router      /api/v1/users/update [post]
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
	if res := a.DB.Model(&existing).Updates(map[string]any{
		"username":  updated.Username,
		"email":     updated.Email,
		"full_name": updated.FullName,
		"role":      updated.Role,
		"active":    updated.Active,
	}); res.Error != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape("username or email already in use"))
	}
	return c.Redirect().To("/users?message=" + url.QueryEscape("user updated"))
}

// UsersDelete removes a user account.
//
// @Summary     Delete user
// @Description Deletes a user account by ID.
// @Tags        users
// @Accept      application/x-www-form-urlencoded
// @Produce     json
// @Param       id formData int true "User ID"
// @Success     303
// @Security    SessionCookie
// @Router      /api/v1/users/delete [post]
func (a *App) UsersDelete(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, 64)
	if err != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape("invalid user id"))
	}
	var u models.User
	if err := a.DB.First(&u, id).Error; err != nil {
		return c.Redirect().To("/users?error=" + url.QueryEscape("user not found"))
	}
	a.DB.Delete(&u)
	return c.Redirect().To("/users?message=" + url.QueryEscape("user deleted"))
}

// validRoles is the set of accepted role values for User accounts.
// Defined at package level to avoid re-allocating the map on every request.
var validRoles = map[string]bool{"admin": true, "editor": true, "viewer": true}

// userFromCtx parses and validates user fields from a Fiber form context.
func (a *App) userFromCtx(c fiber.Ctx) (models.User, error) {
	username := strings.TrimSpace(c.FormValue("username"))
	email := strings.TrimSpace(c.FormValue("email"))
	fullName := strings.TrimSpace(c.FormValue("full_name"))
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
		Username: username,
		Email:    email,
		FullName: fullName,
		Role:     role,
		Active:   active,
	}, nil
}
