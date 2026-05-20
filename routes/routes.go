// Package routes registers all HTTP routes on a Fiber application instance.
// Centralizing route declarations here keeps main.go thin and makes it easy to
// browse the full routing table in a single file.
package routes

import (
	"time"

	swaggo "github.com/gofiber/contrib/v3/swaggo"
	fiberZap "github.com/gofiber/contrib/v3/zap"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
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

	// ── Static file serving ───────────────────────────────────────────────
	// Serves uploaded files at /uploads/<uuid>/<subdir>/<filename>.
	// The "./uploads" directory is created automatically on first upload.
	fApp.Use("/uploads", static.New("./uploads"))

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

	// Digital Signature Form
	fApp.Get("/handover/sign", h.HandoverSignGet)
	fApp.Post("/handover/sign", h.HandoverSignPost)

	fApp.Get("/handover/sign/preview", func(c fiber.Ctx) error {
		return c.Render("handover_sign", fiber.Map{
			"Title": "Asset Handover Form",
			"Token": "preview-token",
			"Form": fiber.Map{
				"LendingLog": fiber.Map{
					"Asset": fiber.Map{
						"Name":         "MacBook Pro 14\"",
						"SerialNumber": "SN-MBP-2024-001",
					},
					"Assignee": fiber.Map{
						"FullName": "Kevin Pratama",
					},
					"LentAt": time.Now(),
				},
			},
		})
	})

	// ── Protected routes (require valid session) ──────────────────────────
	auth := fApp.Group("/", h.AuthRequired)

	// ── Location Master (was Room Master) ────────────────────────────────────────
	auth.Get("/locations", h.LocationsIndex)
	auth.Post("/locations/create", h.LocationsCreate)
	auth.Post("/locations/update", h.LocationsUpdate)
	auth.Post("/locations/delete", h.LocationsDelete)
	auth.Get("/locations/detail", h.LocationDetailsIndex)

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
	auth.Post("/users/create", h.UsersCreate)
	auth.Get("/users/edit", h.UsersEdit)
	auth.Post("/users/update", h.UsersUpdate)
	auth.Post("/users/delete", h.UsersDelete)

	// Assignees
	auth.Get("/assignees", h.AssigneesIndex)
	auth.Get("/assignees/detail", h.AssigneeDetailsIndex)
	auth.Post("/assignees/create", h.AssigneesCreate)
	auth.Post("/assignees/update", h.AssigneesUpdate)
	auth.Post("/assignees/delete", h.AssigneesDelete)

	// Lending Workflow
	auth.Post("/lending/lend", h.LendAsset)
	auth.Post("/lending/return", h.ReturnAsset)
	auth.Get("/handover/receipt", h.HandoverReceiptDownload)

}
