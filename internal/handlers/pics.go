package handlers

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/ABB-Broker/asset-management/internal/models"
	"github.com/gofiber/fiber/v3"
)

// ─────────────────────────────────────────────────────────────────────────────
// PIC (Person In Charge) handlers
// ─────────────────────────────────────────────────────────────────────────────

// PICSIndex renders the PIC list page.
// GET /pics
func (a *App) PICSIndex(c fiber.Ctx) error {
	var pics []models.PIC
	a.DB.Preload("Asset").Preload("User").Order("pic_no asc").Find(&pics)
	var assets []models.Asset
	a.DB.Where("deleted_at IS NULL").Order("asset_no asc").Find(&assets)
	var users []models.User
	a.DB.Where("active = ? AND deleted_at IS NULL", true).Order("user_no asc").Find(&users)
	return c.Render("pics", fiber.Map{
		"Title":       "Person In Charge",
		"CurrentPath": "/pics",
		"Message":     c.Query("message"),
		"Error":       c.Query("error"),
		"PICs":        pics,
		"Assets":      assets,
		"Users":       users,
	})
}

// PICSCreate creates a new PIC record.
// POST /pics/create
func (a *App) PICSCreate(c fiber.Ctx) error {
	assetNo, err := strconv.ParseUint(c.FormValue("asset_no"), 10, 64)
	if err != nil || assetNo == 0 {
		return c.Redirect().To("/pics?error=" + url.QueryEscape("invalid asset"))
	}
	userNo, err := strconv.ParseUint(c.FormValue("user_no"), 10, 64)
	if err != nil || userNo == 0 {
		return c.Redirect().To("/pics?error=" + url.QueryEscape("invalid user"))
	}
	notes := strings.TrimSpace(c.FormValue("notes"))
	pic := models.PIC{AssetNo: uint(assetNo), UserNo: uint(userNo), Notes: notes}
	if err := a.DB.Create(&pic).Error; err != nil {
		return c.Redirect().To("/pics?error=" + url.QueryEscape("failed to create PIC record"))
	}
	return c.Redirect().To("/pics?message=" + url.QueryEscape("PIC created successfully"))
}

// PICSUpdate updates an existing PIC record.
// POST /pics/update
func (a *App) PICSUpdate(c fiber.Ctx) error {
	picNo, err := strconv.ParseUint(c.FormValue("pic_no"), 10, 64)
	if err != nil || picNo == 0 {
		return c.Redirect().To("/pics?error=" + url.QueryEscape("invalid PIC id"))
	}
	userNo, err := strconv.ParseUint(c.FormValue("user_no"), 10, 64)
	if err != nil || userNo == 0 {
		return c.Redirect().To("/pics?error=" + url.QueryEscape("invalid user"))
	}
	notes := strings.TrimSpace(c.FormValue("notes"))
	if err := a.DB.Model(&models.PIC{}).Where("pic_no = ?", picNo).Updates(map[string]any{
		"user_no": userNo,
		"notes":   notes,
	}).Error; err != nil {
		return c.Redirect().To("/pics?error=" + url.QueryEscape("failed to update PIC record"))
	}
	return c.Redirect().To("/pics?message=" + url.QueryEscape("PIC updated successfully"))
}

// PICSDelete deletes a PIC record.
// POST /pics/delete
func (a *App) PICSDelete(c fiber.Ctx) error {
	picNo, err := strconv.ParseUint(c.FormValue("pic_no"), 10, 64)
	if err != nil || picNo == 0 {
		return c.Redirect().To("/pics?error=" + url.QueryEscape("invalid PIC No"))
	}
	if err := a.DB.Delete(&models.PIC{}, picNo).Error; err != nil {
		return c.Redirect().To("/pics?error=" + url.QueryEscape("failed to delete PIC record"))
	}
	return c.Redirect().To("/pics?message=" + url.QueryEscape("PIC deleted successfully"))
}
