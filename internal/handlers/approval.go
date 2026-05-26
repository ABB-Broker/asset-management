package handlers

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/ABB-Broker/asset-management/internal/models"
)

// ApprovalReviewGet renders the PIC's approval page.
// This page is publicly accessible via the approval_token sent in the email,
// so the PIC does not need to be logged in.
//
// GET /approval/review?token=...
func (a *App) ApprovalReviewGet(c fiber.Ctx) error {
	token := c.Query("token")
	if token == "" {
		return c.Status(fiber.StatusBadRequest).SendString("missing approval token")
	}

	var req models.ApprovalRequest
	if err := a.DB.
		Preload("Approver").
		Preload("LendingLog").
		Preload("LendingLog.Asset").
		Preload("LendingLog.Asset.Category").
		Preload("LendingLog.Assignee").
		Preload("LendingLog.HandoverForm").
		Where("approval_token = ?", token).
		First(&req).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("approval request not found")
	}

	if req.Status != "pending" {
		return c.Render("approval_already_decided", fiber.Map{
			"Title":  "Already Decided",
			"Status": req.Status,
		})
	}

	return c.Render("approval_review", fiber.Map{
		"Title":   "Approve Asset Borrow",
		"Request": req,
		"Token":   token,
	})
}

// ApprovalDecidePost processes the PIC's approve or reject decision.
//
// If approved:
//   - Optionally saves the PIC's signature
//   - Generates the receipt PDF
//   - Marks the handover form as "published"
//   - Transitions lending_log.status to "active"
//   - Notifies the borrower
//
// If rejected:
//   - Marks the lending log status back to a terminal state
//   - Notifies the borrower
//
// POST /approval/decide
func (a *App) ApprovalDecidePost(c fiber.Ctx) error {
	token := strings.TrimSpace(c.FormValue("token"))
	decision := strings.TrimSpace(c.FormValue("decision")) // "approved" or "rejected"
	signatureData := strings.TrimSpace(c.FormValue("signature_data"))
	notes := strings.TrimSpace(c.FormValue("notes"))

	if token == "" {
		return c.Status(fiber.StatusBadRequest).SendString("missing token")
	}
	if decision != "approved" && decision != "rejected" {
		return c.Redirect().To("/approval/review?token=" + url.QueryEscape(token) +
			"&error=" + url.QueryEscape("invalid decision value"))
	}

	var req models.ApprovalRequest
	if err := a.DB.
		Preload("Approver").
		Preload("LendingLog").
		Preload("LendingLog.Asset").
		Preload("LendingLog.Asset.Category").
		Preload("LendingLog.Assignee").
		Preload("LendingLog.HandoverForm").
		Where("approval_token = ?", token).
		First(&req).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("approval request not found")
	}

	if req.Status != "pending" {
		return c.Render("approval_already_decided", fiber.Map{
			"Title":  "Already Decided",
			"Status": req.Status,
		})
	}

	now := time.Now()

	// Persist the PIC's decision.
	updates := map[string]any{
		"status":     decision,
		"decided_at": &now,
		"notes":      notes,
	}
	if signatureData != "" {
		updates["signature_data"] = signatureData
	}
	a.DB.Model(&req).Updates(updates)

	if decision == "approved" {
		// Wire the back-reference so generateAndPublishReceipt can access
		// Asset, Assignee, and Category without an extra DB query.
		if req.LendingLog.HandoverForm != nil {
			req.LendingLog.HandoverForm.LendingLog = req.LendingLog
		}
		// Generate the receipt and flip the form + lending log to active.
		a.generateAndPublishReceipt(req.LendingLog.HandoverForm, &req, now)

		// Notify the borrower — but only if they are a linked internal user.
		if req.LendingLog.Assignee.UserNo != nil {
			a.createNotification(
				*req.LendingLog.Assignee.UserNo,
				"approval_decided",
				"Borrow request approved",
				fmt.Sprintf("Your request to borrow %s has been approved by %s. Your receipt has been sent to your email.",
					req.LendingLog.Asset.Name, req.Approver.FullName),
				"lending_log",
				&req.LendingLogNo,
			)
		}

		return c.Render("approval_success", fiber.Map{
			"Title":    "Approved",
			"Decision": "approved",
			"Request":  req,
		})
	}

	// Rejected: mark lending log so it's no longer blocking new borrows.
	// We reuse the "returned" status here to close the cycle cleanly.
	// Alternatively you can add a "rejected" value to the enum in a future migration.
	a.DB.Model(&models.LendingLog{}).
		Where("lending_log_no = ?", req.LendingLogNo).
		Update("status", "returned")

	// Notify the borrower if they are a linked internal user.
	if req.LendingLog.Assignee.UserNo != nil {
		a.createNotification(
			*req.LendingLog.Assignee.UserNo,
			"approval_decided",
			"Borrow request rejected",
			fmt.Sprintf("Your request to borrow %s has been rejected by %s.%s",
				req.LendingLog.Asset.Name,
				req.Approver.FullName,
				notesClause(notes),
			),
			"lending_log",
			&req.LendingLogNo,
		)
	}

	return c.Render("approval_success", fiber.Map{
		"Title":    "Rejected",
		"Decision": "rejected",
		"Request":  req,
	})
}

// notesClause formats a rejection note for inclusion in a notification body.
func notesClause(notes string) string {
	if notes == "" {
		return ""
	}
	return " Reason: " + notes
}
