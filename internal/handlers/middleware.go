package handlers

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/ABB-Broker/asset-management/internal/models"
)

// AuthRequired is a Fiber middleware that ensures the request carries a valid
// fully-authenticated session. Unauthenticated requests are redirected to /login.
func (a *App) AuthRequired(c fiber.Ctx) error {
	token := c.Cookies("session_id")
	if token == "" {
		return c.Redirect().To("/login")
	}

	var sess models.Session
	err := a.DB.Where(
		"token = ? AND authenticated = ? AND expires_at > ?",
		token, true, time.Now(),
	).First(&sess).Error
	if err != nil {
		return c.Redirect().To("/login")
	}

	c.Locals("username", sess.Username)
	return c.Next()
}
