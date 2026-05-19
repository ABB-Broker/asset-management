package handlers

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/ABB-Broker/asset-management/internal/models"
)

// CategoriesIndex renders the Category Master list with an inline create form.
func (a *App) CategoriesIndex(c fiber.Ctx) error {
	var cats []models.Category
	a.DB.Order("id asc").Find(&cats)
	return c.Render("categories", fiber.Map{
		"Title":       "Category Master",
		"CurrentPath": "/categories",
		"Message":     c.Query("message"),
		"Error":       c.Query("error"),
		"Categories":  cats,
		"Category":    models.Category{},
	})
}

// CategoriesCreate persists a new category.
func (a *App) CategoriesCreate(c fiber.Ctx) error {
	name := strings.TrimSpace(c.FormValue("name"))
	desc := strings.TrimSpace(c.FormValue("description"))
	if name == "" {
		return c.Redirect().To("/categories?error=" + url.QueryEscape("category name is required"))
	}
	a.DB.Create(&models.Category{Name: name, Description: desc})
	return c.Redirect().To("/categories?message=" + url.QueryEscape("category created"))
}

// CategoriesEdit renders the edit form pre-filled with the category's current data.
func (a *App) CategoriesEdit(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Query("id"), 10, 64)
	if err != nil {
		return c.Redirect().To("/categories?error=" + url.QueryEscape("invalid category id"))
	}
	var cat models.Category
	if err := a.DB.First(&cat, id).Error; err != nil {
		return c.Redirect().To("/categories?error=" + url.QueryEscape("category not found"))
	}
	var cats []models.Category
	a.DB.Order("id asc").Find(&cats)
	return c.Render("categories", fiber.Map{
		"Title":       "Category Master",
		"CurrentPath": "/categories",
		"Category":    cat,
		"Categories":  cats,
	})
}

// CategoriesUpdate saves changes to an existing category.
func (a *App) CategoriesUpdate(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, 64)
	if err != nil {
		return c.Redirect().To("/categories?error=" + url.QueryEscape("invalid category id"))
	}
	name := strings.TrimSpace(c.FormValue("name"))
	desc := strings.TrimSpace(c.FormValue("description"))
	if name == "" {
		return c.Redirect().To("/categories?error=" + url.QueryEscape("category name is required"))
	}
	var cat models.Category
	if err := a.DB.First(&cat, id).Error; err != nil {
		return c.Redirect().To("/categories?error=" + url.QueryEscape("category not found"))
	}
	a.DB.Model(&cat).Updates(map[string]any{"name": name, "description": desc})
	return c.Redirect().To("/categories?message=" + url.QueryEscape("category updated"))
}

// CategoriesDelete removes a category and all its assets (via ON DELETE CASCADE).
func (a *App) CategoriesDelete(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, 64)
	if err != nil {
		return c.Redirect().To("/categories?error=" + url.QueryEscape("invalid category id"))
	}
	var cat models.Category
	if err := a.DB.First(&cat, id).Error; err != nil {
		return c.Redirect().To("/categories?error=" + url.QueryEscape("category not found"))
	}
	// Cascade deletion of associated assets is handled by the database-level
	// ON DELETE CASCADE constraint (enabled via PRAGMA foreign_keys = ON for SQLite).
	a.DB.Delete(&cat)
	return c.Redirect().To("/categories?message=" + url.QueryEscape("category deleted"))
}
