package handlers

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/ABB-Broker/asset-management/internal/models"
)

// AssetsIndex renders the Asset Master list with an inline create form.
func (a *App) AssetsIndex(c fiber.Ctx) error {
	var assets []models.Asset
	a.DB.Preload("Category").Preload("Room").Order("id asc").Find(&assets)
	var cats []models.Category
	a.DB.Order("id asc").Find(&cats)
	var rooms []models.Room
	a.DB.Order("id asc").Find(&rooms)
	return c.Render("assets", fiber.Map{
		"Title":       "Asset Master",
		"CurrentPath": "/assets",
		"Message":     c.Query("message"),
		"Error":       c.Query("error"),
		"Assets":      assets,
		"Categories":  cats,
		"Asset":       models.Asset{},
		"Rooms":       rooms,
	})
}

// AssetDetailsIndex renders the room details page
func (a *App) AssetDetailsIndex(c fiber.Ctx) error {
	var asset models.Asset
	err := a.DB.Preload("Category").Preload("Room").Preload("AssetPhotos").Where("id = ?", c.Query("id")).First(&asset).Error

	if err != nil {
		return err
	}

	return c.Render("asset_details", fiber.Map{
		"Asset": asset,
	})
}

// AssetsCreate persists a new asset.
func (a *App) AssetsCreate(c fiber.Ctx) error {
	asset, err := a.assetFromCtx(c)
	if err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape(err.Error()))
	}
	a.DB.Create(&asset)
	return c.Redirect().To("/assets?message=" + url.QueryEscape("asset created"))
}

// AssetsEdit renders the edit form pre-filled with the asset's current data.
func (a *App) AssetsEdit(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Query("id"), 10, 64)
	if err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("invalid asset id"))
	}
	var asset models.Asset
	if err := a.DB.Preload("Category").First(&asset, id).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("asset not found"))
	}
	var cats []models.Category
	a.DB.Order("id asc").Find(&cats)
	return c.Render("assets", fiber.Map{
		"Title":       "Asset Master",
		"CurrentPath": "/assets",
		"Message":     c.Query("message"),
		"Error":       c.Query("error"),
		"Assets":      []models.Asset{},
		"Categories":  cats,
		"Asset":       asset,
	})
}

// AssetsUpdate saves changes to an existing asset.
func (a *App) AssetsUpdate(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, 64)
	if err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("invalid asset id"))
	}
	var existing models.Asset
	if err := a.DB.First(&existing, id).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("asset not found"))
	}
	updated, err := a.assetFromCtx(c)
	if err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape(err.Error()))
	}
	a.DB.Model(&existing).Updates(map[string]any{
		"name":          updated.Name,
		"category_id":   updated.CategoryID,
		"serial_number": updated.SerialNumber,
		"purchase_date": updated.PurchaseDate,
	})
	return c.Redirect().To("/assets?message=" + url.QueryEscape("asset updated"))
}

// AssetsDelete removes an asset.
func (a *App) AssetsDelete(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, 64)
	if err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("invalid asset id"))
	}
	var asset models.Asset
	if err := a.DB.First(&asset, id).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("asset not found"))
	}
	a.DB.Delete(&asset)
	return c.Redirect().To("/assets?message=" + url.QueryEscape("asset deleted"))
}

// assetFromCtx parses and validates asset fields from a Fiber form context.
func (a *App) assetFromCtx(c fiber.Ctx) (models.Asset, error) {
	name := strings.TrimSpace(c.FormValue("name"))
	serial := strings.TrimSpace(c.FormValue("serial_number"))
	purchaseDate := strings.TrimSpace(c.FormValue("purchase_date"))
	purchasePrice := strings.TrimSpace(c.FormValue("purchase_price"))
	description := strings.TrimSpace(c.FormValue("description"))
	// Atoi returns int (same bit-width as uint on all supported 64-bit and
	// 32-bit platforms). The uint() cast is a same-size reinterpretation —
	// not a narrowing conversion — so no data loss can occur for valid IDs.
	catIDInt, err := strconv.Atoi(c.FormValue("category_id"))
	if err != nil || catIDInt <= 0 {
		return models.Asset{}, fmt.Errorf("invalid category id")
	}
	categoryID := uint(catIDInt)

	roomIDInt, err := strconv.Atoi(c.FormValue("room_id"))
	if err != nil || roomIDInt <= 0 {
		return models.Asset{}, fmt.Errorf("invalid room_id")
	}
	roomID := uint(roomIDInt)

	if name == "" || serial == "" || purchaseDate == "" || purchasePrice == "" {
		return models.Asset{}, fmt.Errorf("all asset fields are required")
	}

	var cat models.Category
	if err := a.DB.First(&cat, categoryID).Error; err != nil {
		return models.Asset{}, fmt.Errorf("category not found")
	}

	price64, err := strconv.ParseUint(purchasePrice, 10, 32)
	if err != nil {
		return models.Asset{}, fmt.Errorf("invalid purchase price")
	}

	assetModel := &models.Asset{
		Name:          name,
		CategoryID:    categoryID,
		RoomID:        roomID,
		SerialNumber:  serial,
		PurchaseDate:  purchaseDate,
		PurchasePrice: uint(price64),
	}

	if len(description) > 0 {
		assetModel.Description = description
	}

	return *assetModel, nil
}
