package main

import (
	"math/bits"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

func (a *App) categoriesIndex(c fiber.Ctx) error {
	var cats []Category
	a.db.Order("id asc").Find(&cats)
	return c.Render("categories", fiber.Map{
		"Title":       "Category Master",
		"CurrentPath": "/categories",
		"Message":     c.Query("message"),
		"Error":       c.Query("error"),
		"Categories":  cats,
		"Category":    Category{},
	})
}

func (a *App) categoriesCreate(c fiber.Ctx) error {
	name := strings.TrimSpace(c.FormValue("name"))
	desc := strings.TrimSpace(c.FormValue("description"))
	if name == "" {
		return c.Redirect().To("/categories?error=" + url.QueryEscape("category name is required"))
	}
	a.db.Create(&Category{Name: name, Description: desc})
	return c.Redirect().To("/categories?message=" + url.QueryEscape("category created"))
}

func (a *App) categoriesEdit(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Query("id"), 10, bits.UintSize)
	if err != nil {
		return c.Redirect().To("/categories?error=" + url.QueryEscape("invalid category id"))
	}
	var cat Category
	if err := a.db.First(&cat, id).Error; err != nil {
		return c.Redirect().To("/categories?error=" + url.QueryEscape("category not found"))
	}
	var cats []Category
	a.db.Order("id asc").Find(&cats)
	return c.Render("categories", fiber.Map{
		"Title":       "Category Master",
		"CurrentPath": "/categories",
		"Category":    cat,
		"Categories":  cats,
	})
}

func (a *App) categoriesUpdate(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, bits.UintSize)
	if err != nil {
		return c.Redirect().To("/categories?error=" + url.QueryEscape("invalid category id"))
	}
	name := strings.TrimSpace(c.FormValue("name"))
	desc := strings.TrimSpace(c.FormValue("description"))
	if name == "" {
		return c.Redirect().To("/categories?error=" + url.QueryEscape("category name is required"))
	}
	var cat Category
	if err := a.db.First(&cat, id).Error; err != nil {
		return c.Redirect().To("/categories?error=" + url.QueryEscape("category not found"))
	}
	a.db.Model(&cat).Updates(map[string]any{"name": name, "description": desc})
	return c.Redirect().To("/categories?message=" + url.QueryEscape("category updated"))
}

func (a *App) categoriesDelete(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, bits.UintSize)
	if err != nil {
		return c.Redirect().To("/categories?error=" + url.QueryEscape("invalid category id"))
	}
	var cat Category
	if err := a.db.First(&cat, id).Error; err != nil {
		return c.Redirect().To("/categories?error=" + url.QueryEscape("category not found"))
	}
	// Cascade deletion of associated assets is handled by the database-level
	// ON DELETE CASCADE constraint (enabled via PRAGMA foreign_keys = ON for SQLite).
	a.db.Delete(&cat)
	return c.Redirect().To("/categories?message=" + url.QueryEscape("category deleted"))
}
