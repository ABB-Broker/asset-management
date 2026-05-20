package handlers

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/ABB-Broker/asset-management/internal/models"
)

// AssigneesIndex renders the Assignee list page.
func (a *App) AssigneesIndex(c fiber.Ctx) error {
	var assignees []models.Assignee
	a.DB.Preload("User").Order("id asc").Find(&assignees)

	return c.Render("assignees", fiber.Map{
		"Title":       "Assignees",
		"CurrentPath": "/assignees",
		"Message":     c.Query("message"),
		"Error":       c.Query("error"),
		"Assignees":   assignees,
		"Assignee":    models.Assignee{},
	})
}

// AssigneeDetailsIndex renders the details page for a single assignee,
// including their lending history.
func (a *App) AssigneeDetailsIndex(c fiber.Ctx) error {
	var assignee models.Assignee
	if err := a.DB.
		Preload("User").
		Preload("LendingLogs").
		Preload("LendingLogs.Asset").
		Preload("LendingLogs.HandoverForm").
		Where("assignee_uuid = ?", c.Query("uuid")).
		First(&assignee).Error; err != nil {
		return c.Redirect().To("/assignees?error=" + url.QueryEscape("assignee not found"))
	}

	return c.Render("assignee_details", fiber.Map{
		"Title":       assignee.FullName,
		"CurrentPath": "/assignees",
		"Assignee":    assignee,
	})
}

// AssigneesCreate creates a new external assignee (internal ones are auto-created via UsersCreate).
func (a *App) AssigneesCreate(c fiber.Ctx) error {
	assignee, err := a.assigneeFromCtx(c)
	if err != nil {
		return c.Redirect().To("/assignees?error=" + url.QueryEscape(err.Error()))
	}

	if res := a.DB.Create(&assignee); res.Error != nil {
		return c.Redirect().To("/assignees?error=" + url.QueryEscape("failed to create assignee (email may already exist)"))
	}

	return c.Redirect().To("/assignees?message=" + url.QueryEscape("assignee created"))
}

// AssigneesUpdate updates an external assignee record.
func (a *App) AssigneesUpdate(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, 64)
	if err != nil || id == 0 {
		return c.Redirect().To("/assignees?error=" + url.QueryEscape("invalid assignee id"))
	}

	var existing models.Assignee
	if err := a.DB.First(&existing, id).Error; err != nil {
		return c.Redirect().To("/assignees?error=" + url.QueryEscape("assignee not found"))
	}

	// Protect internal assignees from being edited here (edit via User Master).
	if existing.UserID != nil {
		return c.Redirect().To("/assignees?error=" + url.QueryEscape("internal assignees must be edited via User Master"))
	}

	updated, err := a.assigneeFromCtx(c)
	if err != nil {
		return c.Redirect().To("/assignees?error=" + url.QueryEscape(err.Error()))
	}

	a.DB.Model(&existing).Updates(map[string]any{
		"full_name":    updated.FullName,
		"email":        updated.Email,
		"phone_number": updated.PhoneNumber,
		"company":      updated.Company,
		"notes":        updated.Notes,
	})

	return c.Redirect().To("/assignees?message=" + url.QueryEscape("assignee updated"))
}

// AssigneesDelete removes an external assignee.
func (a *App) AssigneesDelete(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, 64)
	if err != nil || id == 0 {
		return c.Redirect().To("/assignees?error=" + url.QueryEscape("invalid assignee id"))
	}

	var assignee models.Assignee
	if err := a.DB.First(&assignee, id).Error; err != nil {
		return c.Redirect().To("/assignees?error=" + url.QueryEscape("assignee not found"))
	}

	if assignee.UserID != nil {
		return c.Redirect().To("/assignees?error=" + url.QueryEscape("cannot delete internal assignee — delete their user account instead"))
	}

	a.DB.Delete(&assignee)

	return c.Redirect().To("/assignees?message=" + url.QueryEscape("assignee deleted"))
}

func (a *App) assigneeFromCtx(c fiber.Ctx) (models.Assignee, error) {
	fullName := strings.TrimSpace(c.FormValue("full_name"))
	email := strings.TrimSpace(c.FormValue("email"))
	phoneNumber := strings.TrimSpace(c.FormValue("phone_number"))
	company := strings.TrimSpace(c.FormValue("company"))
	notes := strings.TrimSpace(c.FormValue("notes"))

	if fullName == "" {
		return models.Assignee{}, fiber.NewError(fiber.StatusBadRequest, "full name is required")
	}
	return models.Assignee{
		FullName:    fullName,
		Email:       email,
		PhoneNumber: phoneNumber,
		Company:     company,
		Notes:       notes,
		// UserID is nil → external assignee
	}, nil
}
