// Package handlers contains all HTTP handler implementations and the central
// App struct that carries shared application state.
package handlers

import (
	contribi18n "github.com/gofiber/contrib/v3/i18n"
	"gorm.io/gorm"

	"github.com/ABB-Broker/asset-management/internal/config"
)

// App holds shared application state that is injected into every handler.
type App struct {
	DB         *gorm.DB
	Cfg        config.Config
	AdminHash  []byte
	Translator *contribi18n.I18n
}
