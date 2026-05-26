package handlers

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/ABB-Broker/asset-management/internal/models"
)

// ─────────────────────────────────────────────────────────────────────────────
// Internal helper — used by lending.go and approval.go
// ─────────────────────────────────────────────────────────────────────────────

// createNotification persists a single notification row for the given user.
// referenceType is a string like "lending_log" or "approval_request".
// referenceNo is the primary key of that record (pass nil if not applicable).
//
// Notification kinds in use:
//   - "approval_requested" — sent to PIC when borrower signs
//   - "approval_decided"   — sent to borrower when PIC approves or rejects
//   - "asset_returned"     — sent to PICs when an asset is returned
//   - "lending_overdue"    — sent to PICs when planned_use_at has passed (scheduled job)
func (a *App) createNotification(userNo uint, kind, title, body, referenceType string, referenceNo *uint) {
	n := models.Notification{
		UserNo:        userNo,
		Kind:          kind,
		Title:         title,
		Body:          body,
		ReferenceType: referenceType,
		ReferenceNo:   referenceNo,
	}
	// Silently ignore errors — notifications are best-effort and must not
	// break the primary borrow/approval flow.
	a.DB.Create(&n)
}

// ─────────────────────────────────────────────────────────────────────────────
// HTTP handlers
// ─────────────────────────────────────────────────────────────────────────────

// NotificationsIndex renders the notification inbox for the logged-in user.
// All unread notifications are marked as read on this visit.
//
// GET /notifications
func (a *App) NotificationsIndex(c fiber.Ctx) error {
	currentUser, err := a.currentUserFromCtx(c)
	if err != nil {
		return c.Redirect().To("/login")
	}

	var notifications []models.Notification
	a.DB.
		Where("user_no = ?", currentUser.UserNo).
		Order("created_at DESC").
		Find(&notifications)

	// Mark all unread notifications as read.
	now := time.Now()
	a.DB.Model(&models.Notification{}).
		Where("user_no = ? AND read_at IS NULL", currentUser.UserNo).
		Update("read_at", &now)

	return c.Render("notifications", fiber.Map{
		"Title":         "Notifications",
		"CurrentPath":   "/notifications",
		"Notifications": notifications,
	})
}

// NotificationMarkRead marks a single notification as read.
// Intended for AJAX calls from the notification dropdown.
//
// POST /notifications/:no/read
func (a *App) NotificationMarkRead(c fiber.Ctx) error {
	currentUser, err := a.currentUserFromCtx(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	notificationNo := c.Params("no")
	now := time.Now()

	result := a.DB.Model(&models.Notification{}).
		Where("notification_no = ? AND user_no = ?", notificationNo, currentUser.UserNo).
		Update("read_at", &now)

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "notification not found"})
	}

	return c.JSON(fiber.Map{"ok": true})
}

// NotificationUnreadCount returns the number of unread notifications for the
// logged-in user as JSON. Used by the bell-icon badge in the nav.
//
// GET /notifications/unread-count
func (a *App) NotificationUnreadCount(c fiber.Ctx) error {
	currentUser, err := a.currentUserFromCtx(c)
	if err != nil {
		// Return 0 rather than an error so the badge silently shows nothing.
		return c.JSON(fiber.Map{"count": 0})
	}

	var count int64
	a.DB.Model(&models.Notification{}).
		Where("user_no = ? AND read_at IS NULL", currentUser.UserNo).
		Count(&count)

	return c.JSON(fiber.Map{"count": count})
}

// ─────────────────────────────────────────────────────────────────────────────
// currentUserFromCtx is a helper shared by notification handlers.
// It reads the username set by the auth middleware and loads the User record.
// ─────────────────────────────────────────────────────────────────────────────

func (a *App) currentUserFromCtx(c fiber.Ctx) (*models.User, error) {
	username, ok := c.Locals("username").(string)
	if !ok || username == "" {
		return nil, fiber.ErrUnauthorized
	}

	var user models.User
	if err := a.DB.Where("username = ? AND active = ?", username, true).First(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}
