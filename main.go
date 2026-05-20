// Information Systems Asset Management — entry point.
//
// @title        Information Systems Asset Management API
// @version      1.0
// @description  REST API for the Information Systems Asset Management system — users, categories, and assets.
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
	"encoding/json"
	"html/template"
	"log"
	"os"
	"strconv"
	"time"

	contribi18n "github.com/gofiber/contrib/v3/i18n"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/gofiber/template/html/v2"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/language"

	"github.com/ABB-Broker/asset-management/internal/config"
	"github.com/ABB-Broker/asset-management/internal/database"
	"github.com/ABB-Broker/asset-management/internal/handlers"
	"github.com/ABB-Broker/asset-management/internal/utils"
	"github.com/ABB-Broker/asset-management/routes"

	// Import generated Swagger docs so they are registered on startup.
	_ "github.com/ABB-Broker/asset-management/docs"
)

func main() {
	cfg := config.Load()

	utils.BaseURL = cfg.BaseURL

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

	cwd, _ := os.Getwd()
	log.Println("CWD:", cwd)

	_, err = os.Stat("./templates/login.html")
	log.Println("template check:", err)

	fApp := newFiberApp(h, logger)

	isProduction := cfg.AppEnv == "production"

	if !fiber.IsChild() {
		log.Printf("Information Systems Asset Management starting on :%s (prefork=%v, admin: %s)",
			cfg.Port, isProduction, cfg.AdminUsername)
	}

	log.Fatal(fApp.Listen(":"+cfg.Port, fiber.ListenConfig{
		EnablePrefork:         isProduction,
		DisableStartupMessage: fiber.IsChild(),
	}))
}

// newFiberApp creates the Fiber application and registers all routes.
// Prefork is configured at Listen time so that tests can call fApp.Test()
// without triggering the prefork machinery.
// logger may be nil; when nil a no-op zap logger is used.
func newFiberApp(h *handlers.App, logger *zap.Logger) *fiber.App {

	engine := html.New("./templates", ".html")

	engine.AddFunc("safeHTML", func(s string) template.HTML {
		return template.HTML(s)
	})

	engine.AddFunc("formatPrice", func(v uint) string {
		// Format with period as thousands separator: 1000000 → 1.000.000
		s := strconv.FormatUint(uint64(v), 10)
		n := len(s)
		if n <= 3 {
			return s
		}
		var result []byte
		for i, c := range s {
			if i > 0 && (n-i)%3 == 0 {
				result = append(result, '.')
			}
			result = append(result, byte(c))
		}
		return string(result)
	})

	engine.AddFunc("inc", func(i interface{}) int {
		switch v := i.(type) {
		case int:
			return v + 1
		case int64:
			return int(v) + 1
		case uint:
			return int(v) + 1
		case uint64:
			return int(v) + 1
		default:
			return 0
		}
	})

	engine.AddFunc("formatDate", func(t *time.Time) string {
		if t == nil {
			return "—"
		}
		return t.Format("02 Jan 2006")
	})

	engine.AddFunc("toJSON", func(v interface{}) string {
		b, err := json.MarshalIndent(v, "", " ")
		if err != nil {
			return err.Error()
		}
		return string(b)
	})

	fApp := fiber.New(fiber.Config{
		Views:     engine,
		BodyLimit: 20 * 1024 * 1024,
	})
	fApp.Use("/", static.New("./public"))
	routes.Setup(fApp, h, logger)
	return fApp
}
