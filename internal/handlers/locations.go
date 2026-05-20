package handlers

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/ABB-Broker/asset-management/internal/models"
	"github.com/ABB-Broker/asset-management/internal/utils"
)

// ─── READ ─────────────────────────────────────────────────────────────────────

func (a *App) LocationsIndex(c fiber.Ctx) error {
	var locations []models.Location
	a.DB.Preload("LocationPhotos").Order("id asc").Find(&locations)

	for i := range locations {
		for j := range locations[i].LocationPhotos {
			locations[i].LocationPhotos[j].PhotoUrl = utils.WithBaseURL(locations[i].LocationPhotos[j].PhotoUrl)
		}
	}

	return c.Render("locations", fiber.Map{
		"Title":       "Location Master",
		"CurrentPath": "/locations",
		"Message":     c.Query("message"),
		"Error":       c.Query("error"),
		"Locations":   locations,
		"Location":    models.Location{},
	})
}

func (a *App) LocationDetailsIndex(c fiber.Ctx) error {
	var location models.Location
	if err := a.DB.
		Preload("Assets").
		Preload("Assets.Category").
		Preload("LocationPhotos").
		Where("location_uuid = ?", c.Query("uuid")).
		First(&location).Error; err != nil {
		return c.Redirect().To("/locations?error=" + url.QueryEscape("location not found"))
	}

	for i := range location.LocationPhotos {
		location.LocationPhotos[i].PhotoUrl = utils.WithBaseURL(location.LocationPhotos[i].PhotoUrl)
	}

	return c.Render("location_details", fiber.Map{
		"Title":       location.LocationName,
		"CurrentPath": "/locations",
		"Location":    location,
	})
}

// ─── CREATE ───────────────────────────────────────────────────────────────────

func (a *App) LocationsCreate(c fiber.Ctx) error {
	name := strings.TrimSpace(c.FormValue("name"))
	description := strings.TrimSpace(c.FormValue("description"))

	if name == "" {
		return c.Redirect().To("/locations?error=" + url.QueryEscape("location name is required"))
	}

	location := models.Location{
		LocationName: name,
		Description:  description,
	}

	if err := a.DB.Create(&location).Error; err != nil {
		return c.Redirect().To("/locations?error=" + url.QueryEscape("failed to create location"))
	}

	a.saveNewLocationPhotos(c, &location)

	return c.Redirect().To("/locations?message=" + url.QueryEscape("location created"))
}

// ─── UPDATE ───────────────────────────────────────────────────────────────────

func (a *App) LocationsUpdate(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, 64)
	if err != nil || id == 0 {
		return c.Redirect().To("/locations?error=" + url.QueryEscape("invalid location id"))
	}

	var location models.Location
	if err := a.DB.Preload("LocationPhotos").First(&location, id).Error; err != nil {
		return c.Redirect().To("/locations?error=" + url.QueryEscape("location not found"))
	}

	name := strings.TrimSpace(c.FormValue("name"))
	description := strings.TrimSpace(c.FormValue("description"))
	if name == "" {
		return c.Redirect().To("/locations?error=" + url.QueryEscape("location name is required"))
	}

	a.DB.Model(&location).Updates(map[string]any{
		"location_name": name,
		"description":   description,
	})

	a.deleteLocationPhotos(c, &location)
	a.updateExistingLocationPhotos(c, &location)
	a.saveNewLocationPhotos(c, &location)

	return c.Redirect().To("/locations?message=" + url.QueryEscape("location updated"))
}

// ─── DELETE ───────────────────────────────────────────────────────────────────

func (a *App) LocationsDelete(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, 64)
	if err != nil || id == 0 {
		return c.Redirect().To("/locations?error=" + url.QueryEscape("invalid location id"))
	}

	var location models.Location
	if err := a.DB.Preload("LocationPhotos").First(&location, id).Error; err != nil {
		return c.Redirect().To("/locations?error=" + url.QueryEscape("location not found"))
	}

	var assets []models.Asset
	a.DB.Preload("AssetPhotos").Where("location_id = ?", location.ID).Find(&assets)

	for _, asset := range assets {
		for _, p := range asset.AssetPhotos {
			utils.DeleteFile(p.PhotoUrl)
		}
		a.DB.Delete(&asset)
	}

	for _, p := range location.LocationPhotos {
		utils.DeleteFile(p.PhotoUrl)
	}

	a.DB.Delete(&location)

	return c.Redirect().To("/locations?message=" + url.QueryEscape("location deleted"))
}

// ─── Photo helpers ────────────────────────────────────────────────────────────

func (a *App) saveNewLocationPhotos(c fiber.Ctx, location *models.Location) {
	form, err := c.MultipartForm()
	if err != nil {
		return
	}
	files := form.File["new_photos[]"]
	names := form.Value["new_photo_name[]"]
	for i, fh := range files {
		relativePath, err := utils.SaveFile(fh, location.LocationUUID, "locations")
		if err != nil {
			continue
		}
		photoName := fh.Filename
		if i < len(names) && strings.TrimSpace(names[i]) != "" {
			photoName = strings.TrimSpace(names[i])
		}
		a.DB.Create(&models.LocationPhotos{
			LocationID: location.ID,
			Name:       photoName,
			PhotoUrl:   relativePath,
		})
	}
}

func (a *App) deleteLocationPhotos(c fiber.Ctx, location *models.Location) {
	form, err := c.MultipartForm()
	if err != nil {
		return
	}
	for _, rawID := range form.Value["delete_photo[]"] {
		photoID, err := strconv.ParseUint(strings.TrimSpace(rawID), 10, 64)
		if err != nil || photoID == 0 {
			continue
		}
		var photo models.LocationPhotos
		if err := a.DB.First(&photo, photoID).Error; err != nil {
			continue
		}
		if photo.LocationID != location.ID {
			continue
		}
		utils.DeleteFile(photo.PhotoUrl)
		a.DB.Delete(&photo)
	}
}

func (a *App) updateExistingLocationPhotos(c fiber.Ctx, location *models.Location) {
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
		var photo models.LocationPhotos
		if err := a.DB.First(&photo, photoID).Error; err != nil {
			continue
		}
		if photo.LocationID != location.ID {
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
			newPath, err := utils.SaveFile(replaceFiles[0], location.LocationUUID, "locations")
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
