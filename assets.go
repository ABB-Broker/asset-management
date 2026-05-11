package main

import (
	"fmt"
	"math/bits"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

func (a *App) assetsIndex(c fiber.Ctx) error {
	var assets []Asset
	a.db.Preload("Category").Order("id asc").Find(&assets)
	var cats []Category
	a.db.Order("id asc").Find(&cats)
	return c.Render("assets", fiber.Map{
		"Title":       "Asset Master",
		"CurrentPath": "/assets",
		"Message":     c.Query("message"),
		"Error":       c.Query("error"),
		"Assets":      assets,
		"Categories":  cats,
		"Asset":       Asset{},
	})
}

func (a *App) assetsCreate(c fiber.Ctx) error {
	asset, err := a.assetFromCtx(c)
	if err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape(err.Error()))
	}
	a.db.Create(&asset)
	return c.Redirect().To("/assets?message=" + url.QueryEscape("asset created"))
}

func (a *App) assetsEdit(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Query("id"), 10, bits.UintSize)
	if err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("invalid asset id"))
	}
	var asset Asset
	if err := a.db.Preload("Category").First(&asset, id).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("asset not found"))
	}
	var cats []Category
	a.db.Order("id asc").Find(&cats)
	return c.Render("assets", fiber.Map{
		"Title":       "Asset Master",
		"CurrentPath": "/assets",
		"Message":     c.Query("message"),
		"Error":       c.Query("error"),
		"Assets":      []Asset{},
		"Categories":  cats,
		"Asset":       asset,
	})
}

func (a *App) assetsUpdate(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, bits.UintSize)
	if err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("invalid asset id"))
	}
	var existing Asset
	if err := a.db.First(&existing, id).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("asset not found"))
	}
	updated, err := a.assetFromCtx(c)
	if err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape(err.Error()))
	}
	a.db.Model(&existing).Updates(map[string]any{
		"name":          updated.Name,
		"category_id":   updated.CategoryID,
		"serial_number": updated.SerialNumber,
		"purchase_date": updated.PurchaseDate,
	})
	return c.Redirect().To("/assets?message=" + url.QueryEscape("asset updated"))
}

func (a *App) assetsDelete(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, bits.UintSize)
	if err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("invalid asset id"))
	}
	var asset Asset
	if err := a.db.First(&asset, id).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("asset not found"))
	}
	a.db.Delete(&asset)
	return c.Redirect().To("/assets?message=" + url.QueryEscape("asset deleted"))
}

// assetFromCtx parses and validates asset fields from a Fiber form context.
func (a *App) assetFromCtx(c fiber.Ctx) (Asset, error) {
	name := strings.TrimSpace(c.FormValue("name"))
	serial := strings.TrimSpace(c.FormValue("serial_number"))
	purchaseDate := strings.TrimSpace(c.FormValue("purchase_date"))
	categoryID, err := strconv.ParseUint(c.FormValue("category_id"), 10, bits.UintSize)
	if err != nil || categoryID == 0 {
		return Asset{}, fmt.Errorf("invalid category id")
	}
	if name == "" || serial == "" || purchaseDate == "" {
		return Asset{}, fmt.Errorf("all asset fields are required")
	}
	var cat Category
	if err := a.db.First(&cat, categoryID).Error; err != nil {
		return Asset{}, fmt.Errorf("category not found")
	}
	return Asset{
		Name:         name,
		CategoryID:   uint(categoryID),
		SerialNumber: serial,
		PurchaseDate: purchaseDate,
	}, nil
}
