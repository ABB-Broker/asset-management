package handlers

import (
	"fmt"
	"time"

	"github.com/ABB-Broker/asset-management/internal/models"
	"github.com/gofiber/fiber/v3"
)

// AuthRequired is a Fiber middleware that ensures the request carries a valid
// fully-authenticated session. Unauthenticated requests are redirected to /login.
func (a *App) AuthRequired(c fiber.Ctx) error {
	token := c.Cookies("session_id")
	if token == "" {
		return c.Redirect().To("/login")
	}
	var sess models.Session
	err := a.DB.Where("token = ? AND authenticated = ? AND expires_at > ?", token, true, time.Now()).First(&sess).Error
	if err != nil {
		return c.Redirect().To("/login")
	}

	userNo := sess.UserNo

	var user models.User
	if err := a.DB.Where("user_no = ?", userNo).First(&user).Error; err != nil {
		fmt.Printf("Getting users error: %v\n", err)
	}
	c.Locals("username", user.Username)
	return c.Next()
}

// OptionalAuth populates c.Locals("username") if the user has a valid session,
// but always continues to the next handler even if unauthenticated.
// Use this for pages that are publicly accessible but show extra actions when logged in.
func (a *App) OptionalAuth(c fiber.Ctx) error {
	token := c.Cookies("session_id")

	if token != "" {
		var sess models.Session
		if err := a.DB.Where("token = ? AND authenticated = ? AND expires_at > ?", token, true, time.Now()).First(&sess).Error; err == nil {
			userNo := sess.UserNo

			var user models.User
			if err := a.DB.Where("user_no = ?", userNo).First(&user).Error; err != nil {
				fmt.Printf("Getting users error: %v\n", err)
			}

			c.Locals("username", user.Username)
		}

	}
	return c.Next()
}
