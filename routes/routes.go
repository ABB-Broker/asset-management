// Package routes registers all HTTP routes on a Fiber application instance.
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
func Setup(fApp *fiber.App, h *handlers.App, logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}
	fApp.Use(fiberZap.New(fiberZap.Config{
		Logger:   logger,
		SkipURIs: []string{"/swagger/*"},
	}))

	// ── Static file serving ───────────────────────────────────────────────
	fApp.Use("/uploads", static.New("./uploads"))

	// ── Swagger UI ────────────────────────────────────────────────────────
	fApp.Get("/swagger/*", swaggo.HandlerDefault)

	// ── Public routes ─────────────────────────────────────────────────────
	fApp.Get("/", func(c fiber.Ctx) error { return c.Redirect().To("/login") })
	fApp.Get("/login", h.LoginGet)
	fApp.Post("/login", h.LoginPost)
	fApp.Get("/login/2fa", h.Login2FAGet)
	fApp.Post("/login/2fa", h.Login2FAPost)
	fApp.Get("/logout", h.Logout)
	// fApp.Post("/admin/create", h.UsersCreate)

	// Change password
	fApp.Get("/change-password", h.ChangePasswordGet)
	fApp.Post("/change-password", h.ChangePasswordPost)

	// ── Set-password / invite flow (no auth required) ─────────────────────
	fApp.Get("/set-password", h.SetPasswordGet)
	fApp.Post("/set-password", h.SetPasswordPost)

	// ── Forgot / reset password (no auth required) ───────────────────────
	fApp.Get("/forgot-password", h.ForgotPasswordGet)
	fApp.Post("/forgot-password", h.ForgotPasswordPost)

	// ── Digital Signature Form (no auth required) ─────────────────────────
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

	// PIC approval pages (token-authenticated, same pattern as /handover/sign)
	fApp.Get("/approval/review", h.ApprovalReviewGet)
	fApp.Post("/approval/decide", h.ApprovalDecidePost)

	// ── QR-code public pages (OptionalAuth: shows actions if logged in) ───
	qr := fApp.Group("/qr", h.OptionalAuth)
	qr.Get("/assets/detail", h.AssetDetailsPublic)
	qr.Get("/locations/detail", h.LocationDetailsPublic)

	// ── Protected routes (require valid session) ──────────────────────────
	auth := fApp.Group("/", h.AuthRequired)

	// Location Master
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
	auth.Post("/assets/update", h.AssetsUpdate)
	auth.Post("/assets/delete", h.AssetsDelete)
	auth.Get("/assets/detail", h.AssetDetailsIndex)

	// User Master
	auth.Get("/users", h.UsersIndex)
	auth.Post("/users/create", h.UsersCreate)
	auth.Get("/users/edit", h.UsersEdit)
	auth.Post("/users/update", h.UsersUpdate)
	auth.Post("/users/delete", h.UsersDelete)
	auth.Post("/users/resend-invite", h.UsersResendInvite)

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

	// PIC (Person In Charge)
	auth.Get("/pics", h.PICSIndex)
	auth.Post("/pics/create", h.PICSCreate)
	auth.Post("/pics/update", h.PICSUpdate)
	auth.Post("/pics/delete", h.PICSDelete)

	// Notifications
	auth.Get("/notifications", h.NotificationsIndex)
	auth.Get("/notifications/unread-count", h.NotificationUnreadCount)
	auth.Post("/notifications/:no/read", h.NotificationMarkRead)
}
