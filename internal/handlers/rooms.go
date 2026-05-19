package handlers

import (
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

// RoomsIndex renders the Room Master list with an inline create/edit form.
func (a *App) RoomsIndex(c fiber.Ctx) error {
	var rooms []models.Room
	a.DB.
		Preload("RoomPhotos").Order("id asc").Find(&rooms)

	for i := range rooms {
		for j := range rooms[i].RoomPhotos {
			rooms[i].RoomPhotos[j].PhotoUrl = utils.WithBaseURL(rooms[i].RoomPhotos[j].PhotoUrl)
		}
	}

	return c.Render("rooms", fiber.Map{
		"Title":       "Room Master",
		"CurrentPath": "/rooms",
		"Message":     c.Query("message"),
		"Error":       c.Query("error"),
		"Rooms":       rooms,
		"Room":        models.Room{},
	})
}

// RoomDetailsIndex renders the room details page.
// Stored photo paths are converted to full URLs before being passed to the
// template so the frontend receives ready-to-use image src values.
func (a *App) RoomDetailsIndex(c fiber.Ctx) error {
	var room models.Room
	if err := a.DB.
		Preload("Assets").
		Preload("Assets.Category").
		Preload("RoomPhotos").
		Where("room_uuid = ?", c.Query("uuid")).
		First(&room).Error; err != nil {
		return c.Redirect().To("/rooms?error=" + url.QueryEscape("room not found"))
	}

	// Convert relative paths → full URLs for the template.
	for i := range room.RoomPhotos {
		room.RoomPhotos[i].PhotoUrl = utils.WithBaseURL(room.RoomPhotos[i].PhotoUrl)
	}

	return c.Render("room_details", fiber.Map{
		"Title":       room.RoomName,
		"CurrentPath": "/rooms",
		"Room":        room,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// CREATE
// ─────────────────────────────────────────────────────────────────────────────

// RoomsCreate persists a new room and saves any attached photos to disk.
func (a *App) RoomsCreate(c fiber.Ctx) error {
	name := strings.TrimSpace(c.FormValue("name"))
	description := strings.TrimSpace(c.FormValue("description"))

	if name == "" {
		return c.Redirect().To("/rooms?error=" + url.QueryEscape("room name is required"))
	}

	room := models.Room{
		RoomName:    name,
		Description: description,
		// RoomUUID is set automatically by the BeforeCreate GORM hook.
	}

	if err := a.DB.Create(&room).Error; err != nil {
		return c.Redirect().To("/rooms?error=" + url.QueryEscape("failed to create room"))
	}

	// Persist any newly-uploaded photos.
	a.saveNewRoomPhotos(c, &room)

	return c.Redirect().To("/rooms?message=" + url.QueryEscape("room created"))
}

// ─────────────────────────────────────────────────────────────────────────────
// UPDATE
// ─────────────────────────────────────────────────────────────────────────────

// RoomsUpdate saves changes to an existing room, including photo additions,
// replacements, renames and deletions.
func (a *App) RoomsUpdate(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, 64)
	if err != nil || id == 0 {
		return c.Redirect().To("/rooms?error=" + url.QueryEscape("invalid room id"))
	}

	var room models.Room
	if err := a.DB.Preload("RoomPhotos").First(&room, id).Error; err != nil {
		return c.Redirect().To("/rooms?error=" + url.QueryEscape("room not found"))
	}

	name := strings.TrimSpace(c.FormValue("name"))
	description := strings.TrimSpace(c.FormValue("description"))
	if name == "" {
		return c.Redirect().To("/rooms?error=" + url.QueryEscape("room name is required"))
	}

	// 1. Update scalar fields.
	a.DB.Model(&room).Updates(map[string]any{
		"room_name":   name,
		"description": description,
	})

	// 2. Delete photos that were marked for removal.
	a.deleteRoomPhotos(c, &room)

	// 3. Rename / replace existing photos.
	a.updateExistingRoomPhotos(c, &room)

	// 4. Add brand-new photos.
	a.saveNewRoomPhotos(c, &room)

	return c.Redirect().To("/rooms?message=" + url.QueryEscape("room updated"))
}

// ─────────────────────────────────────────────────────────────────────────────
// DELETE
// ─────────────────────────────────────────────────────────────────────────────

// RoomsDelete removes a room and all its associated data.
// Uploaded photo files are removed from disk before the DB row is deleted.
func (a *App) RoomsDelete(c fiber.Ctx) error {
	id, err := strconv.ParseUint(c.FormValue("id"), 10, 64)
	if err != nil || id == 0 {
		return c.Redirect().To("/rooms?error=" + url.QueryEscape("invalid room id"))
	}

	var room models.Room
	if err := a.DB.Preload("RoomPhotos").First(&room, id).Error; err != nil {
		return c.Redirect().To("/rooms?error=" + url.QueryEscape("room not found"))
	}

	// Load assets with their photos so we can remove files from disk.
	var assets []models.Asset
	a.DB.Preload("AssetPhotos").Where("room_id = ?", room.ID).Find(&assets)

	// Remove each asset's photo files, then delete the asset.
	for _, asset := range assets {
		for _, p := range asset.AssetPhotos {
			utils.DeleteFile(p.PhotoUrl)
		}
		a.DB.Delete(&asset)
	}

	// Remove room photo files from disk.
	for _, p := range room.RoomPhotos {
		utils.DeleteFile(p.PhotoUrl)
	}

	// The ON DELETE CASCADE constraint removes RoomPhotos rows automatically.
	a.DB.Delete(&room)

	return c.Redirect().To("/rooms?message=" + url.QueryEscape("room deleted"))
}

// ─────────────────────────────────────────────────────────────────────────────
// Photo helper methods
// ─────────────────────────────────────────────────────────────────────────────

// saveNewRoomPhotos reads `new_photos[]` and `new_photo_name[]` from the
// multipart form and saves each file to disk, creating a RoomPhotos DB row.
func (a *App) saveNewRoomPhotos(c fiber.Ctx, room *models.Room) {
	form, err := c.MultipartForm()
	if err != nil {
		return // no multipart body — nothing to do
	}

	files := form.File["new_photos[]"]
	names := form.Value["new_photo_name[]"]

	for i, fh := range files {
		relativePath, err := utils.SaveFile(fh, room.RoomUUID, "rooms")
		if err != nil {
			continue // skip unreadable uploads; log in production
		}

		photoName := fh.Filename
		if i < len(names) && strings.TrimSpace(names[i]) != "" {
			photoName = strings.TrimSpace(names[i])
		}

		a.DB.Create(&models.RoomPhotos{
			RoomID:   room.ID,
			Name:     photoName,
			PhotoUrl: relativePath,
		})
	}
}

// deleteRoomPhotos processes the `delete_photo[]` form values, removing the
// corresponding files from disk and deleting their DB rows.
func (a *App) deleteRoomPhotos(c fiber.Ctx, room *models.Room) {
	form, err := c.MultipartForm()
	if err != nil {
		return
	}

	for _, rawID := range form.Value["delete_photo[]"] {
		photoID, err := strconv.ParseUint(strings.TrimSpace(rawID), 10, 64)
		if err != nil || photoID == 0 {
			continue
		}

		var photo models.RoomPhotos
		if err := a.DB.First(&photo, photoID).Error; err != nil {
			continue
		}
		// Only delete photos that belong to this room.
		if photo.RoomID != room.ID {
			continue
		}

		utils.DeleteFile(photo.PhotoUrl)
		a.DB.Delete(&photo)
	}
}

// updateExistingRoomPhotos processes:
//   - `existing_photo_name[{id}]` — rename a photo
//   - `replace_photo[{id}]`       — swap the file while keeping the DB row
func (a *App) updateExistingRoomPhotos(c fiber.Ctx, room *models.Room) {
	form, err := c.MultipartForm()
	if err != nil {
		return
	}

	// Gather all photo IDs referenced in the form.
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
		var photo models.RoomPhotos
		if err := a.DB.First(&photo, photoID).Error; err != nil {
			continue
		}
		if photo.RoomID != room.ID {
			continue
		}

		updates := map[string]any{}

		// Rename?
		nameKey := "existing_photo_name[" + strconv.FormatUint(uint64(photoID), 10) + "]"
		if newName := strings.TrimSpace(form.Value[nameKey][0]); newName != "" && newName != photo.Name {
			updates["name"] = newName
		}

		// Replace file?
		replaceKey := "replace_photo[" + strconv.FormatUint(uint64(photoID), 10) + "]"
		if replaceFiles, ok := form.File[replaceKey]; ok && len(replaceFiles) > 0 {
			newPath, err := utils.SaveFile(replaceFiles[0], room.RoomUUID, "rooms")
			if err == nil {
				utils.DeleteFile(photo.PhotoUrl) // remove the old file
				updates["photo_url"] = newPath
			}
		}

		if len(updates) > 0 {
			a.DB.Model(&photo).Updates(updates)
		}
	}
}
