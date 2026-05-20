package handlers

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/ABB-Broker/asset-management/internal/models"
	"github.com/ABB-Broker/asset-management/internal/utils"
)

// ─────────────────────────────────────────────────────────────────────────────
// READ
// ─────────────────────────────────────────────────────────────────────────────

// AssetsIndex renders the Asset Master list with an inline create/edit form.
func (a *App) AssetsIndex(c fiber.Ctx) error {
	var assets []models.Asset
	a.DB.Preload("Category").Preload("Location").Preload("AssetPhotos").Order("id asc").Find(&assets)
	var cats []models.Category
	a.DB.Order("id asc").Find(&cats)
	var locations []models.Location
	a.DB.Order("id asc").Find(&locations)

	for i := range assets {
		for j := range assets[i].AssetPhotos {
			assets[i].AssetPhotos[j].PhotoUrl = utils.WithBaseURL(assets[i].AssetPhotos[j].PhotoUrl)
		}
	}

	return c.Render("assets", fiber.Map{
		"Title":       "Asset Master",
		"CurrentPath": "/assets",
		"Message":     c.Query("message"),
		"Error":       c.Query("error"),
		"Assets":      assets,
		"Categories":  cats,
		"Locations":   locations,
		"Asset":       models.Asset{},
	})
}

// AssetDetailsIndex renders the asset details page.
// Stored photo paths are converted to full URLs before being passed to the
// template so the frontend receives ready-to-use image src values.
func (a *App) AssetDetailsIndex(c fiber.Ctx) error {
	var asset models.Asset
	var cats []models.Category
	a.DB.Order("id asc").Find(&cats)
	var locations []models.Location
	a.DB.Order("id asc").Find(&locations)
	if err := a.DB.
		Preload("Category").
		Preload("Location"). // ← was Room
		Preload("AssetPhotos").
		Preload("LendingLogs").              // ← new
		Preload("LendingLogs.Assignee").     // ← new
		Preload("LendingLogs.HandoverForm"). // ← new
		Where("asset_uuid = ?", c.Query("uuid")).
		First(&asset).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("asset not found"))
	}

	// Convert relative paths → full URLs for the template.
	for i := range asset.AssetPhotos {
		asset.AssetPhotos[i].PhotoUrl = utils.WithBaseURL(asset.AssetPhotos[i].PhotoUrl)
	}

	var assignees []models.Assignee
	a.DB.Order("id asc").Find(&assignees)

	var currentUser *models.User
	if username, ok := c.Locals("username").(string); ok && username != "" {
		var u models.User
		if err := a.DB.Where("username = ? AND active = ?", username, true).First(&u).Error; err == nil {
			currentUser = &u
		}
	}

	var currentAssigneeID uint
	if currentUser != nil && currentUser.AssigneeID != nil {
		currentAssigneeID = *currentUser.AssigneeID
	}

	return c.Render("asset_details", fiber.Map{
		"Title":             asset.Name,
		"CurrentPath":       "/assets",
		"Asset":             asset,
		"Categories":        cats,
		"Locations":         locations,
		"Assignees":         assignees,
		"CurrentUser":       currentUser,
		"CurrentAssigneeID": currentAssigneeID, // 0 if not applicable

	})
}

// AssetsEdit renders the edit form pre-filled with the asset's current data.
// Existing photo URLs are converted to full URLs so the edit template can
// render previews.
func (a *App) AssetsEdit(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Query("id"), 10, 64)
	if err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("invalid asset id"))
	}

	var asset models.Asset
	if err := a.DB.Preload("Category").Preload("AssetPhotos").First(&asset, id).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("asset not found"))
	}

	// Convert stored paths → full URLs so the template can render previews.
	for i := range asset.AssetPhotos {
		asset.AssetPhotos[i].PhotoUrl = utils.WithBaseURL(asset.AssetPhotos[i].PhotoUrl)
	}

	var cats []models.Category
	a.DB.Order("id asc").Find(&cats)
	var locations []models.Location
	a.DB.Order("id asc").Find(&locations)

	return c.Render("assets", fiber.Map{
		"Title":       "Asset Master",
		"CurrentPath": "/assets",
		"Message":     c.Query("message"),
		"Error":       c.Query("error"),
		"Assets":      []models.Asset{},
		"Categories":  cats,
		"Locations":   locations,
		"Asset":       asset,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// CREATE
// ─────────────────────────────────────────────────────────────────────────────

// AssetsCreate persists a new asset and saves any attached photos to disk.
func (a *App) AssetsCreate(c fiber.Ctx) error {
	asset, err := a.assetFromCtx(c)
	if err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape(err.Error()))
	}

	// AssetUUID is set automatically by the BeforeCreate GORM hook.
	if err := a.DB.Create(&asset).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("failed to create asset"))
	}

	// Persist any newly-uploaded photos.
	a.saveNewAssetPhotos(c, &asset)

	return c.Redirect().To("/assets?message=" + url.QueryEscape("asset created"))
}

// ─────────────────────────────────────────────────────────────────────────────
// UPDATE
// ─────────────────────────────────────────────────────────────────────────────

// AssetsUpdate saves changes to an existing asset, including photo additions,
// replacements, renames and deletions.
func (a *App) AssetsUpdate(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, 64)
	if err != nil || id == 0 {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("invalid asset id"))
	}

	var existing models.Asset
	if err := a.DB.Preload("AssetPhotos").First(&existing, id).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("asset not found"))
	}

	updated, err := a.assetFromCtx(c)
	if err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape(err.Error()))
	}

	// 1. Update scalar fields.
	a.DB.Model(&existing).Updates(map[string]any{
		"name":           updated.Name,
		"description":    updated.Description,
		"asset_type":     updated.AssetType,
		"category_id":    updated.CategoryID,
		"location_id":    updated.LocationID,
		"serial_number":  updated.SerialNumber,
		"purchase_date":  updated.PurchaseDate,
		"purchase_price": updated.PurchasePrice,
	})

	// 2. Delete photos that were marked for removal.
	a.deleteAssetPhotos(c, &existing)

	// 3. Rename / replace existing photos.
	a.updateExistingAssetPhotos(c, &existing)

	// 4. Add brand-new photos.
	a.saveNewAssetPhotos(c, &existing)

	return c.Redirect().To("/assets?message=" + url.QueryEscape("asset updated"))
}

// ─────────────────────────────────────────────────────────────────────────────
// DELETE
// ─────────────────────────────────────────────────────────────────────────────

// AssetsDelete removes an asset and its associated photo files from disk.
func (a *App) AssetsDelete(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, 64)
	if err != nil || id == 0 {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("invalid asset id"))
	}

	var asset models.Asset
	if err := a.DB.Preload("AssetPhotos").First(&asset, id).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("asset not found"))
	}

	// Remove photo files from disk first.
	for _, p := range asset.AssetPhotos {
		utils.DeleteFile(p.PhotoUrl)
	}

	// ON DELETE CASCADE removes AssetPhotos rows automatically.
	a.DB.Delete(&asset)

	return c.Redirect().To("/assets?message=" + url.QueryEscape("asset deleted"))
}

// ─────────────────────────────────────────────────────────────────────────────
// Form parsing helper
// ─────────────────────────────────────────────────────────────────────────────

// assetFromCtx parses and validates asset fields from a Fiber form context.
func (a *App) assetFromCtx(c fiber.Ctx) (models.Asset, error) {
	name := strings.TrimSpace(c.FormValue("name"))
	serial := strings.TrimSpace(c.FormValue("serial_number"))
	purchaseDate := strings.TrimSpace(c.FormValue("purchase_date"))
	purchasePrice := strings.TrimSpace(c.FormValue("purchase_price"))
	description := strings.TrimSpace(c.FormValue("description"))

	catIDInt, err := strconv.Atoi(c.FormValue("category_id"))
	if err != nil || catIDInt <= 0 {
		return models.Asset{}, fmt.Errorf("invalid category id")
	}
	categoryID := uint(catIDInt)

	assetType := strings.TrimSpace(c.FormValue("asset_type"))
	if assetType != "fixed" && assetType != "movable" {
		assetType = "fixed"
	}

	// Replace lines 254–261 with:
	var locationID *uint
	if raw := strings.TrimSpace(c.FormValue("location_id")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			u := uint(n)
			locationID = &u
		}
	}
	if assetType == "fixed" && locationID == nil {
		return models.Asset{}, fmt.Errorf("location is required for fixed assets")
	}

	if name == "" || serial == "" || purchaseDate == "" || purchasePrice == "" {
		return models.Asset{}, fmt.Errorf("all asset fields are required")
	}

	var cat models.Category
	if err := a.DB.First(&cat, categoryID).Error; err != nil {
		return models.Asset{}, fmt.Errorf("category not found")
	}

	purchasePrice = strings.ReplaceAll(purchasePrice, ".", "")
	purchasePrice = strings.ReplaceAll(purchasePrice, ",", "")
	price64, err := strconv.ParseUint(purchasePrice, 10, 32)
	if err != nil {
		return models.Asset{}, fmt.Errorf("invalid purchase price")
	}

	m := models.Asset{
		Name:          name,
		Description:   description,
		AssetType:     assetType,
		CategoryID:    categoryID,
		LocationID:    locationID,
		SerialNumber:  serial,
		PurchaseDate:  purchaseDate,
		PurchasePrice: uint(price64),
	}
	return m, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Photo helper methods
// ─────────────────────────────────────────────────────────────────────────────

// saveNewAssetPhotos reads `new_photos[]` and `new_photo_name[]` from the
// multipart form and saves each file to disk, creating an AssetPhotos DB row.
func (a *App) saveNewAssetPhotos(c fiber.Ctx, asset *models.Asset) {
	form, err := c.MultipartForm()
	if err != nil {
		return
	}

	files := form.File["new_photos[]"]
	names := form.Value["new_photo_name[]"]

	for i, fh := range files {
		relativePath, err := utils.SaveFile(fh, asset.AssetUUID, "assets")
		if err != nil {
			continue
		}

		photoName := fh.Filename
		if i < len(names) && strings.TrimSpace(names[i]) != "" {
			photoName = strings.TrimSpace(names[i])
		}

		a.DB.Create(&models.AssetPhotos{
			AssetID:  asset.ID,
			Name:     photoName,
			PhotoUrl: relativePath,
		})
	}
}

// deleteAssetPhotos processes the `delete_photo[]` form values, removing the
// corresponding files from disk and deleting their DB rows.
func (a *App) deleteAssetPhotos(c fiber.Ctx, asset *models.Asset) {
	form, err := c.MultipartForm()
	if err != nil {
		return
	}

	for _, rawID := range form.Value["delete_photo[]"] {
		photoID, err := strconv.ParseUint(strings.TrimSpace(rawID), 10, 64)
		if err != nil || photoID == 0 {
			continue
		}

		var photo models.AssetPhotos
		if err := a.DB.First(&photo, photoID).Error; err != nil {
			continue
		}
		if photo.AssetID != asset.ID {
			continue
		}

		utils.DeleteFile(photo.PhotoUrl)
		a.DB.Delete(&photo)
	}
}

// updateExistingAssetPhotos processes:
//   - `existing_photo_name[{id}]` — rename a photo
//   - `replace_photo[{id}]`       — swap the file while keeping the DB row
func (a *App) updateExistingAssetPhotos(c fiber.Ctx, asset *models.Asset) {
	form, err := c.MultipartForm()
	if err != nil {
		return
	}

	// Collect all photo IDs referenced in the form.
	photoIDs := map[uint]struct{}{}
	for key := range form.Value {
		if strings.HasPrefix(key, "existing_photo_name[") {
			rawID := strings.TrimSuffix(strings.TrimPrefix(key, "existing_photo_name["), "]")
			if id, err := strconv.ParseUint(rawID, 10, 64); err == nil && id > 0 {
				photoIDs[uint(id)] = struct{}{}
			}
		}
	}
	for key := range form.File {
		if strings.HasPrefix(key, "replace_photo[") {
			rawID := strings.TrimSuffix(strings.TrimPrefix(key, "replace_photo["), "]")
			if id, err := strconv.ParseUint(rawID, 10, 64); err == nil && id > 0 {
				photoIDs[uint(id)] = struct{}{}
			}
		}
	}

	for photoID := range photoIDs {
		var photo models.AssetPhotos
		if err := a.DB.First(&photo, photoID).Error; err != nil {
			continue
		}
		if photo.AssetID != asset.ID {
			continue
		}

		updates := map[string]any{}

		// Rename?
		nameKey := "existing_photo_name[" + strconv.FormatUint(uint64(photoID), 10) + "]"
		if vals := form.Value[nameKey]; len(vals) > 0 {
			if newName := strings.TrimSpace(vals[0]); newName != "" && newName != photo.Name {
				updates["name"] = newName
			}
		}

		// Replace file?
		replaceKey := "replace_photo[" + strconv.FormatUint(uint64(photoID), 10) + "]"
		if replaceFiles, ok := form.File[replaceKey]; ok && len(replaceFiles) > 0 {
			newPath, err := utils.SaveFile(replaceFiles[0], asset.AssetUUID, "assets")
			if err == nil {
				utils.DeleteFile(photo.PhotoUrl)
				updates["photo_url"] = newPath
			}
		}

		if len(updates) > 0 {
			a.DB.Model(&photo).Updates(updates)
		}
	}
}
