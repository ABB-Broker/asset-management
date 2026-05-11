package main

import (
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/template/html/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// App holds shared application state available to all handlers.
type App struct {
	db        *gorm.DB
	cfg       Config
	adminHash []byte
}

func main() {
	cfg := loadConfig()

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("bcrypt: %v", err)
	}

	db := initDB(cfg)
	handler := &App{db: db, cfg: cfg, adminHash: hash}
	fApp := newFiberApp(handler)

	if !fiber.IsChild() {
		log.Printf("Asset management starting on :%s (prefork=%v, admin: %s)", cfg.Port, cfg.Prefork, cfg.AdminUsername)
	}
	log.Fatal(fApp.Listen(":"+cfg.Port, fiber.ListenConfig{
		EnablePrefork:         cfg.Prefork,
		DisableStartupMessage: fiber.IsChild(),
	}))
}

// newFiberApp creates and configures the Fiber application with all routes.
// Prefork is configured at Listen time (via main) rather than here so that
// tests can call fApp.Test() without triggering the prefork machinery.
func newFiberApp(handler *App) *fiber.App {
	engine := html.New("./templates", ".html")

	fApp := fiber.New(fiber.Config{
		Views: engine,
	})

	// Public routes
	fApp.Get("/", func(c fiber.Ctx) error {
		return c.Redirect().To("/login")
	})
	fApp.Get("/login", handler.loginGet)
	fApp.Post("/login", handler.loginPost)
	fApp.Get("/login/2fa", handler.login2FAGet)
	fApp.Post("/login/2fa", handler.login2FAPost)
	fApp.Get("/logout", handler.logout)

	// Protected routes — guarded by authRequired middleware
	auth := fApp.Group("/", handler.authRequired)
	auth.Get("/categories", handler.categoriesIndex)
	auth.Post("/categories/create", handler.categoriesCreate)
	auth.Get("/categories/edit", handler.categoriesEdit)
	auth.Post("/categories/update", handler.categoriesUpdate)
	auth.Post("/categories/delete", handler.categoriesDelete)
	auth.Get("/assets", handler.assetsIndex)
	auth.Post("/assets/create", handler.assetsCreate)
	auth.Get("/assets/edit", handler.assetsEdit)
	auth.Post("/assets/update", handler.assetsUpdate)
	auth.Post("/assets/delete", handler.assetsDelete)

	return fApp
}
