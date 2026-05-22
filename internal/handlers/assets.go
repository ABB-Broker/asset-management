package handlers

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"github.com/ABB-Broker/asset-management/internal/models"
	"github.com/ABB-Broker/asset-management/internal/utils"
)

// ─────────────────────────────────────────────────────────────────────────────
// READ
// ─────────────────────────────────────────────────────────────────────────────

// AssetsIndex renders the Asset Master list with an inline create/edit form.
func (a *App) AssetsIndex(c fiber.Ctx) error {
	var assets []models.Asset
	a.DB.Preload("Category").Preload("Location").Preload("AssetPhotos").Order("asset_no asc").Find(&assets)
	var cats []models.Category
	a.DB.Order("category_no asc").Find(&cats)
	var locations []models.Location
	a.DB.Order("location_no asc").Find(&locations)

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
	a.DB.Order("category_no asc").Find(&cats)
	var locations []models.Location
	a.DB.Order("location_no asc").Find(&locations)
	if err := a.DB.
		Preload("Category").
		Preload("Location"). // ← was Room
		Preload("AssetPhotos").
		Preload("LendingLogs").                                                               // ← new
		Preload("LendingLogs.Assignee", func(db *gorm.DB) *gorm.DB { return db.Unscoped() }). // ← new
		Preload("LendingLogs.HandoverForm").                                                  // ← new
		Preload("PICs").
		Preload("PICs.User").
		Where("asset_uuid = ?", c.Query("uuid")).
		First(&asset).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("asset not found"))
	}

	// Convert relative paths → full URLs for the template.
	for i := range asset.AssetPhotos {
		asset.AssetPhotos[i].PhotoUrl = utils.WithBaseURL(asset.AssetPhotos[i].PhotoUrl)
	}

	var assignees []models.Assignee
	a.DB.Order("assignee_no asc").Find(&assignees)

	var currentUser *models.User
	if username, ok := c.Locals("username").(string); ok && username != "" {
		var u models.User
		if err := a.DB.Where("username = ? AND active = ?", username, true).First(&u).Error; err == nil {
			currentUser = &u
		}
	}

	var currentAssigneeID uint
	if currentUser != nil && currentUser.AssigneeNo != nil {
		currentAssigneeID = *currentUser.AssigneeNo
	}

	var users []models.User
	a.DB.Where("active = ? AND deleted_at IS NULL", true).Order("user_no asc").Find(&users)

	return c.Render("asset_details", fiber.Map{
		"Title":             asset.Name,
		"CurrentPath":       "/assets",
		"Asset":             asset,
		"Categories":        cats,
		"Locations":         locations,
		"Assignees":         assignees,
		"Users":             users,
		"CurrentUser":       currentUser,
		"CurrentAssigneeID": currentAssigneeID, // 0 if not applicable

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
		"category_no":    updated.CategoryNo,
		"location_no":    updated.LocationNo,
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

	catIDInt, err := strconv.Atoi(c.FormValue("category_no"))
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
	if raw := strings.TrimSpace(c.FormValue("location_no")); raw != "" {
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
		CategoryNo:    categoryID,
		LocationNo:    locationID,
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

		a.DB.Create(&models.AssetPhoto{
			AssetNo:  asset.AssetNo,
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

		var photo models.AssetPhoto
		if err := a.DB.First(&photo, photoID).Error; err != nil {
			continue
		}
		if photo.AssetNo != asset.AssetNo {
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
		var photo models.AssetPhoto
		if err := a.DB.First(&photo, photoID).Error; err != nil {
			continue
		}
		if photo.AssetNo != asset.AssetNo {
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

// AssetDetailsPublic renders a read-only asset details page for QR-code visitors.
// GET /qr/assets/detail?uuid=...
// Uses OptionalAuth: if the visitor is logged in, actions are still available.
func (a *App) AssetDetailsPublic(c fiber.Ctx) error {
	var asset models.Asset
	var cats []models.Category
	a.DB.Order("category_no asc").Find(&cats)
	var locations []models.Location
	a.DB.Order("location_no asc").Find(&locations)
	if err := a.DB.
		Preload("Category").
		Preload("Location").
		Preload("AssetPhotos").
		Preload("LendingLogs").
		Preload("LendingLogs.Assignee", func(db *gorm.DB) *gorm.DB { return db.Unscoped() }).
		Preload("LendingLogs.HandoverForm").
		Preload("PICs").
		Preload("PICs.User").
		Where("asset_uuid = ?", c.Query("uuid")).
		First(&asset).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("asset not found")
	}

	for i := range asset.AssetPhotos {
		asset.AssetPhotos[i].PhotoUrl = utils.WithBaseURL(asset.AssetPhotos[i].PhotoUrl)
	}

	var assignees []models.Assignee
	a.DB.Order("assignee_no asc").Find(&assignees)

	// Check whether the visitor is logged in (OptionalAuth sets this).
	username, _ := c.Locals("username").(string)
	isLoggedIn := username != ""

	var currentUser *models.User
	var currentAssigneeID uint
	if isLoggedIn {
		var u models.User
		if err := a.DB.Where("username = ? AND active = ?", username, true).First(&u).Error; err == nil {
			currentUser = &u
			if u.AssigneeNo != nil {
				currentAssigneeID = *u.AssigneeNo
			}
		}
	}

	return c.Render("asset_details_public", fiber.Map{
		"Title":             asset.Name + " — Asset Details",
		"Asset":             asset,
		"Categories":        cats,
		"Locations":         locations,
		"Assignees":         assignees,
		"IsLoggedIn":        isLoggedIn,
		"CurrentUser":       currentUser,
		"CurrentAssigneeID": currentAssigneeID,
	})
}

// LocationDetailsPublic renders a read-only location details page for QR-code visitors.
// GET /qr/locations/detail?uuid=...
// Uses OptionalAuth: if the visitor is logged in, actions are still available.
func (a *App) LocationDetailsPublic(c fiber.Ctx) error {
	var location models.Location
	if err := a.DB.Preload("Assets").Preload("Assets.Category").Preload("LocationPhotos").Where("location_uuid = ?", c.Query("uuid")).First(&location).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("location not found")
	}
	for i := range location.LocationPhotos {
		location.LocationPhotos[i].PhotoUrl = utils.WithBaseURL(location.LocationPhotos[i].PhotoUrl)
	}
	username, _ := c.Locals("username").(string)
	isLoggedIn := username != ""
	return c.Render("location_details_public", fiber.Map{"Title": location.LocationName + " — Location Details", "Location": location, "IsLoggedIn": isLoggedIn})
}
