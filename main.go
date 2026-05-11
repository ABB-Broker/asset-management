package main

import (
	"log"

	contribi18n "github.com/gofiber/contrib/v3/i18n"
	swaggo "github.com/gofiber/contrib/v3/swaggo"
	fiberZap "github.com/gofiber/contrib/v3/zap"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/template/html/v2"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/language"
	"gorm.io/gorm"

	// Import generated Swagger docs so they are registered on startup.
	_ "github.com/ABB-Broker/asset-management/docs"
)

// App holds shared application state available to all handlers.
type App struct {
	db          *gorm.DB
	cfg         Config
	adminHash   []byte
	translator  *contribi18n.I18n
}

func main() {
	cfg := loadConfig()

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("bcrypt: %v", err)
	}

	db := initDB(cfg)

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("zap logger init: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	translator := contribi18n.New(&contribi18n.Config{
		RootPath:        "./localize",
		AcceptLanguages: []language.Tag{language.English, language.Indonesian},
		DefaultLanguage: language.English,
	})

	handler := &App{db: db, cfg: cfg, adminHash: hash, translator: translator}
	fApp := newFiberApp(handler, logger)

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
// logger may be nil in tests; when nil a no-op zap logger is used.
func newFiberApp(handler *App, logger *zap.Logger) *fiber.App {
	engine := html.New("./templates", ".html")

	fApp := fiber.New(fiber.Config{
		Views: engine,
	})

	// ── Zap structured logger middleware ──────────────────────────────────
	if logger == nil {
		logger = zap.NewNop()
	}
	fApp.Use(fiberZap.New(fiberZap.Config{
		Logger:   logger,
		SkipURIs: []string{"/swagger/*"},
	}))

	// ── Swagger UI ────────────────────────────────────────────────────────
	fApp.Get("/swagger/*", swaggo.HandlerDefault)

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

	// User Master routes
	auth.Get("/users", handler.usersIndex)
	auth.Post("/users/create", handler.usersCreate)
	auth.Get("/users/edit", handler.usersEdit)
	auth.Post("/users/update", handler.usersUpdate)
	auth.Post("/users/delete", handler.usersDelete)

	return fApp
}
