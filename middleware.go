package main

import (
	"time"

	"github.com/gofiber/fiber/v3"
)

// authRequired is a Fiber middleware that ensures the request carries a valid
// fully-authenticated session. Unauthenticated requests are redirected to /login.
func (a *App) authRequired(c fiber.Ctx) error {
	token := c.Cookies("session_id")
	if token == "" {
		return c.Redirect().To("/login")
	}

	var sess Session
	err := a.db.Where(
		"token = ? AND authenticated = ? AND expires_at > ?",
		token, true, time.Now(),
	).First(&sess).Error
	if err != nil {
		return c.Redirect().To("/login")
	}

	c.Locals("username", sess.Username)
	return c.Next()
}
