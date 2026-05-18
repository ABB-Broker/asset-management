package handlers

import (
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/ABB-Broker/asset-management/internal/models"
)

// RoomsIndex renders the Room Master list with an inline create form.
func (a *App) RoomsIndex(c fiber.Ctx) error {
	var rooms []models.Room
	a.DB.Order("id asc").Find(&rooms)
	return c.Render("rooms", fiber.Map{
		"Title":       "Room Master",
		"CurrentPath": "/rooms",
		"Message":     c.Query("message"),
		"Error":       c.Query("error"),
		"Rooms":       rooms,
		"Category":    models.Room{},
	})
}

// RoomDetailsIndex renders the room details page
func (a *App) RoomDetailsIndex(c fiber.Ctx) error {
	var room models.Room
	a.DB.Preload("Assets").Preload("Assets.Category").Preload("RoomPhotos").Where("id = ?", c.Query("id")).First(&room)

	return c.Render("room_details", fiber.Map{
		"Room": room,
	})
}

// RoomsCreate persists a new room.
func (a *App) RoomsCreate(c fiber.Ctx) error {
	room, err := a.roomFromCtx(c)
	if err != nil {
		return c.Redirect().To("/rooms?error=" + url.QueryEscape(err.Error()))
	}
	a.DB.Create(&room)
	return c.Redirect().To("/rooms?message=" + url.QueryEscape("asset created"))
}

// roomFromCtx parses and validates asset fields from a Fiber form context.
func (a *App) roomFromCtx(c fiber.Ctx) (models.Room, error) {
	name := strings.TrimSpace(c.FormValue("name"))
	description := strings.TrimSpace(c.FormValue("description"))

	roomModel := &models.Room{
		RoomName: name,
	}

	if len(description) > 0 {
		roomModel.Description = description
	}

	return *roomModel, nil
}
