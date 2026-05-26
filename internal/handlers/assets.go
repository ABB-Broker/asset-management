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
func (a *App) AssetDetailsIndex(c fiber.Ctx) error {
	var asset models.Asset
	var cats []models.Category
	a.DB.Order("category_no asc").Find(&cats)
	var locations []models.Location
	a.DB.Order("location_no asc").Find(&locations)

	if err := a.DB.
		Preload("Category").
		Preload("Location").
		Preload("AssetPhotos").
		Preload("LendingLogs", func(db *gorm.DB) *gorm.DB {
			return db.Order("lending_log_no DESC")
		}).
		Preload("LendingLogs.Assignee", func(db *gorm.DB) *gorm.DB { return db.Unscoped() }).
		Preload("LendingLogs.HandoverForm").
		Preload("LendingLogs.ApprovalRequest").
		Preload("PICs").
		Preload("PICs.User").
		Where("asset_uuid = ?", c.Query("uuid")).
		First(&asset).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("asset not found"))
	}

	for i := range asset.AssetPhotos {
		asset.AssetPhotos[i].PhotoUrl = utils.WithBaseURL(asset.AssetPhotos[i].PhotoUrl)
	}

	// Find the currently active/pending lending log (if any) to show the
	// "currently borrowed" badge and disable the lend form in the template.
	var activeLending *models.LendingLog
	for i := range asset.LendingLogs {
		s := asset.LendingLogs[i].Status
		if s == "pending_signature" || s == "pending_approval" || s == "active" {
			activeLending = &asset.LendingLogs[i]
			break
		}
	}

	var assignees []models.Assignee
	a.DB.Order("assignee_no asc").Find(&assignees)

	// Look up the logged-in user and their linked assignee record.
	// NOTE: We query assignees by user_no — there is no reverse FK on users.
	currentUser, _ := a.currentUserFromCtx(c)
	var currentAssigneeNo uint
	if currentUser != nil {
		var linkedAssignee models.Assignee
		if err := a.DB.Where("user_no = ?", currentUser.UserNo).First(&linkedAssignee).Error; err == nil {
			currentAssigneeNo = linkedAssignee.AssigneeNo
		}
	}

	var users []models.User
	a.DB.Where("active = ? AND deleted_at IS NULL", true).Order("user_no asc").Find(&users)

	// PIC eligibility flags passed to the template.
	// CanLend:       needs >= 1 PIC on the asset.
	// CanBorrowSelf: if current user IS a PIC, needs >= 2 PICs (so another can approve);
	//               otherwise needs >= 1 PIC.
	picCount := len(asset.PICs)
	currentUserIsPIC := false
	if currentUser != nil {
		for _, p := range asset.PICs {
			if p.UserNo == currentUser.UserNo {
				currentUserIsPIC = true
				break
			}
		}
	}
	canLend := picCount >= 1
	canBorrowSelf := false
	if currentAssigneeNo > 0 {
		if currentUserIsPIC {
			canBorrowSelf = picCount >= 2
		} else {
			canBorrowSelf = picCount >= 1
		}
	}

	return c.Render("asset_details", fiber.Map{
		"Title":             asset.Name,
		"CurrentPath":       "/assets",
		"Message":           c.Query("message"),
		"Error":             c.Query("error"),
		"Asset":             asset,
		"Categories":        cats,
		"Locations":         locations,
		"Assignees":         assignees,
		"Users":             users,
		"CurrentUser":       currentUser,
		"CurrentAssigneeNo": currentAssigneeNo, // 0 if not linked
		"ActiveLending":     activeLending,     // nil if asset is free
		"CanLend":           canLend,           // false when no PICs assigned
		"CanBorrowSelf":     canBorrowSelf,     // false when PIC rules not satisfied
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

	if err := a.DB.Create(&asset).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("failed to create asset"))
	}

	a.saveNewAssetPhotos(c, &asset)

	return c.Redirect().To("/assets?message=" + url.QueryEscape("asset created"))
}

// ─────────────────────────────────────────────────────────────────────────────
// UPDATE
// ─────────────────────────────────────────────────────────────────────────────

// AssetsUpdate saves changes to an existing asset.
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

	a.deleteAssetPhotos(c, &existing)
	a.updateExistingAssetPhotos(c, &existing)
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

	for _, p := range asset.AssetPhotos {
		utils.DeleteFile(p.PhotoUrl)
	}

	a.DB.Delete(&asset)

	return c.Redirect().To("/assets?message=" + url.QueryEscape("asset deleted"))
}

// ─────────────────────────────────────────────────────────────────────────────
// Form parsing helper
// ─────────────────────────────────────────────────────────────────────────────

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

	return models.Asset{
		Name:          name,
		Description:   description,
		AssetType:     assetType,
		CategoryNo:    categoryID,
		LocationNo:    locationID,
		SerialNumber:  serial,
		PurchaseDate:  purchaseDate,
		PurchasePrice: uint(price64),
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Photo helpers
// ─────────────────────────────────────────────────────────────────────────────

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

func (a *App) updateExistingAssetPhotos(c fiber.Ctx, asset *models.Asset) {
	form, err := c.MultipartForm()
	if err != nil {
		return
	}

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

		nameKey := "existing_photo_name[" + strconv.FormatUint(uint64(photoID), 10) + "]"
		if vals := form.Value[nameKey]; len(vals) > 0 {
			if newName := strings.TrimSpace(vals[0]); newName != "" && newName != photo.Name {
				updates["name"] = newName
			}
		}

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

// ─────────────────────────────────────────────────────────────────────────────
// Public / QR handlers
// ─────────────────────────────────────────────────────────────────────────────

// AssetDetailsPublic renders a read-only asset details page for QR-code visitors.
//
// GET /qr/assets/detail?uuid=...
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

	username, _ := c.Locals("username").(string)
	isLoggedIn := username != ""

	// NOTE: We no longer use user.AssigneeNo — we query by user_no on assignees.
	var currentUser *models.User
	var currentAssigneeNo uint
	if isLoggedIn {
		var u models.User
		if err := a.DB.Where("username = ? AND active = ?", username, true).First(&u).Error; err == nil {
			currentUser = &u
			var linkedAssignee models.Assignee
			if err := a.DB.Where("user_no = ?", u.UserNo).First(&linkedAssignee).Error; err == nil {
				currentAssigneeNo = linkedAssignee.AssigneeNo
			}
		}
	}

	// Find the currently active/pending lending log (if any).
	var activeLending *models.LendingLog
	for i := range asset.LendingLogs {
		s := asset.LendingLogs[i].Status
		if s == "pending_signature" || s == "pending_approval" || s == "active" {
			activeLending = &asset.LendingLogs[i]
			break
		}
	}

	// PIC eligibility flags (same rules as AssetDetailsIndex).
	picCount := len(asset.PICs)
	currentUserIsPIC := false
	if currentUser != nil {
		for _, p := range asset.PICs {
			if p.UserNo == currentUser.UserNo {
				currentUserIsPIC = true
				break
			}
		}
	}
	canLend := picCount >= 1
	canBorrowSelf := false
	if currentAssigneeNo > 0 {
		if currentUserIsPIC {
			canBorrowSelf = picCount >= 2
		} else {
			canBorrowSelf = picCount >= 1
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
		"CurrentAssigneeNo": currentAssigneeNo,
		"CurrentAssigneeID": currentAssigneeNo,
		"ActiveLending":     activeLending,
		"CanLend":           canLend,
		"CanBorrowSelf":     canBorrowSelf,
	})
}

// LocationDetailsPublic renders a read-only location details page for QR-code visitors.
//
// GET /qr/locations/detail?uuid=...
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
