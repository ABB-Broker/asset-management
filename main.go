// Asset Management — entry point.
//
// @title        Asset Management API
// @version      1.0
// @description  REST API for the Asset Management system — users, categories, and assets.
// @termsOfService http://swagger.io/terms/
//
// @contact.name  ABB-Broker
// @contact.url   https://github.com/ABB-Broker/asset-management
//
// @license.name MIT
// @license.url  https://opensource.org/licenses/MIT
//
// @host      localhost:8080
// @BasePath  /api/v1
//
// @securityDefinitions.apikey SessionCookie
// @in cookie
// @name session_id
package main

import (
	"log"

	contribi18n "github.com/gofiber/contrib/v3/i18n"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/template/html/v2"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/language"

	"github.com/ABB-Broker/asset-management/internal/config"
	"github.com/ABB-Broker/asset-management/internal/database"
	"github.com/ABB-Broker/asset-management/internal/handlers"
	"github.com/ABB-Broker/asset-management/routes"

	// Import generated Swagger docs so they are registered on startup.
	_ "github.com/ABB-Broker/asset-management/docs"
)

func main() {
	cfg := config.Load()

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("bcrypt: %v", err)
	}

	db := database.Init(cfg)

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

	h := &handlers.App{
		DB:         db,
		Cfg:        cfg,
		AdminHash:  hash,
		Translator: translator,
	}

	fApp := newFiberApp(h, logger)

	if !fiber.IsChild() {
		log.Printf("Asset management starting on :%s (prefork=%v, admin: %s)",
			cfg.Port, cfg.Prefork, cfg.AdminUsername)
	}
	log.Fatal(fApp.Listen(":"+cfg.Port, fiber.ListenConfig{
		EnablePrefork:         cfg.Prefork,
		DisableStartupMessage: fiber.IsChild(),
	}))
}

// newFiberApp creates the Fiber application and registers all routes.
// Prefork is configured at Listen time so that tests can call fApp.Test()
// without triggering the prefork machinery.
// logger may be nil; when nil a no-op zap logger is used.
func newFiberApp(h *handlers.App, logger *zap.Logger) *fiber.App {
	engine := html.New("./templates", ".html")
	fApp := fiber.New(fiber.Config{Views: engine})
	routes.Setup(fApp, h, logger)
	return fApp
}
