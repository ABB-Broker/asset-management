// Package routes registers all HTTP routes on a Fiber application instance.
// Centralizing route declarations here keeps main.go thin and makes it easy to
// browse the full routing table in a single file.
package routes

import (
	swaggo "github.com/gofiber/contrib/v3/swaggo"
	fiberZap "github.com/gofiber/contrib/v3/zap"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"

	"github.com/ABB-Broker/asset-management/internal/handlers"
)

// Setup registers all routes and middleware on fApp.
// logger may be nil; when nil a zap.NewNop() logger is used.
func Setup(fApp *fiber.App, h *handlers.App, logger *zap.Logger) {
	// ── Zap structured request logger ────────────────────────────────────
	if logger == nil {
		logger = zap.NewNop()
	}
	fApp.Use(fiberZap.New(fiberZap.Config{
		Logger:   logger,
		SkipURIs: []string{"/swagger/*"},
	}))

	// ── Swagger UI ────────────────────────────────────────────────────────
	fApp.Get("/swagger/*", swaggo.HandlerDefault)

	// ── Public routes ─────────────────────────────────────────────────────
	fApp.Get("/", func(c fiber.Ctx) error {
		return c.Redirect().To("/login")
	})
	fApp.Get("/login", h.LoginGet)
	fApp.Post("/login", h.LoginPost)
	fApp.Get("/login/2fa", h.Login2FAGet)
	fApp.Post("/login/2fa", h.Login2FAPost)
	// fApp.Post("/admin/create", h.UsersCreate)
	fApp.Get("/logout", h.Logout)

	// ── Protected routes (require valid session) ──────────────────────────
	auth := fApp.Group("/", h.AuthRequired)

	// Room Master
	auth.Get("/rooms", h.RoomsIndex)
	auth.Post("/rooms/create", h.RoomsCreate)
	auth.Get("/rooms/detail", h.RoomDetailsIndex)

	// Category Master
	auth.Get("/categories", h.CategoriesIndex)
	auth.Post("/categories/create", h.CategoriesCreate)
	auth.Get("/categories/edit", h.CategoriesEdit)
	auth.Post("/categories/update", h.CategoriesUpdate)
	auth.Post("/categories/delete", h.CategoriesDelete)

	// Asset Master
	auth.Get("/assets", h.AssetsIndex)
	auth.Post("/assets/create", h.AssetsCreate)
	auth.Get("/assets/edit", h.AssetsEdit)
	auth.Post("/assets/update", h.AssetsUpdate)
	auth.Post("/assets/delete", h.AssetsDelete)
	auth.Get("/assets/detail", h.AssetDetailsIndex)

	// User Master
	auth.Get("/users", h.UsersIndex)
	auth.Get("/users/edit", h.UsersEdit)
	auth.Post("/users/update", h.UsersUpdate)
	auth.Post("/users/delete", h.UsersDelete)
}
