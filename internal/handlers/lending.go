package handlers

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/ABB-Broker/asset-management/internal/models"
	"github.com/ABB-Broker/asset-management/internal/utils"
)

// LendAsset assigns a movable asset to an assignee and sends the handover form link via email.
// POST /lending/lend
func (a *App) LendAsset(c fiber.Ctx) error {
	assetID, err := strconv.ParseUint(c.FormValue("asset_no"), 10, 64)
	if err != nil || assetID == 0 {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("invalid asset id"))
	}

	assigneeID, err := strconv.ParseUint(c.FormValue("assignee_no"), 10, 64)
	if err != nil || assigneeID == 0 {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("invalid assignee id"))
	}

	var asset models.Asset
	if err := a.DB.First(&asset, assetID).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("asset not found"))
	}

	if asset.AssetType != "movable" {
		return c.Redirect().To(fmt.Sprintf("/assets/detail?uuid=%s&error=%s",
			asset.AssetUUID, url.QueryEscape("only movable assets can be lent out")))
	}

	var assignee models.Assignee
	if err := a.DB.First(&assignee, assigneeID).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("assignee not found"))
	}

	notes := strings.TrimSpace(c.FormValue("notes"))

	// Parse optional planned-use datetime (format: "2006-01-02T15:04")
	var plannedUseAt *time.Time
	if rawPlanned := strings.TrimSpace(c.FormValue("planned_use_at")); rawPlanned != "" {
		if parsed, parseErr := time.ParseInLocation("2006-01-02T15:04", rawPlanned, time.Local); parseErr == nil {
			plannedUseAt = &parsed
		}
	}

	tx := a.DB.Begin()

	lendingLog := models.LendingLog{
		AssetNo:      uint(assetID),
		AssigneeNo:   uint(assigneeID),
		LentAt:       time.Now(),
		PlannedUseAt: plannedUseAt,
		Status:       "pending_signature",
		Notes:        notes,
	}
	if err := tx.Create(&lendingLog).Error; err != nil {
		tx.Rollback()
		return c.Redirect().To("/assets?error=" + url.QueryEscape("failed to create lending record"))
	}

	now := time.Now()
	handoverForm := models.HandoverForm{
		LendingLogNo: lendingLog.LendingLogNo,
		SentAt:       &now,
		Status:       "sent",
	}
	if err := tx.Create(&handoverForm).Error; err != nil {
		tx.Rollback()
		return c.Redirect().To("/assets?error=" + url.QueryEscape("failed to create handover form"))
	}

	tx.Commit()

	// Send email to assignee with the public signature form link.
	formURL := fmt.Sprintf("%s/handover/sign?token=%s", a.Cfg.BaseURL, handoverForm.FormToken)
	_ = a.sendHandoverEmail(assignee.Email, assignee.FullName, asset.Name, formURL)

	return c.Redirect().To(fmt.Sprintf("/assets/detail?uuid=%s&message=%s",
		asset.AssetUUID, url.QueryEscape("asset lent — handover form sent to "+assignee.Email)))
}

// ReturnAsset marks a lending log as returned.
// POST /lending/return
func (a *App) ReturnAsset(c fiber.Ctx) error {
	lendingID, err := strconv.ParseUint(c.FormValue("lending_no"), 10, 64)
	if err != nil || lendingID == 0 {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("invalid lending id"))
	}

	var log models.LendingLog
	if err := a.DB.Preload("Asset").First(&log, lendingID).Error; err != nil {
		return c.Redirect().To("/assets?error=" + url.QueryEscape("lending record not found"))
	}

	// Parse the returned_at date from the form; fall back to now.
	returnedAt := time.Now()
	if dateStr := strings.TrimSpace(c.FormValue("returned_at")); dateStr != "" {
		if parsed, parseErr := time.ParseInLocation("2006-01-02", dateStr, time.Local); parseErr == nil {
			// Use end-of-day so the timestamp is unambiguous.
			returnedAt = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 23, 59, 59, 0, time.Local)
		}
	}
	a.DB.Model(&log).Updates(map[string]any{
		"returned_at": &returnedAt,
		"status":      "returned",
	})

	return c.Redirect().To(fmt.Sprintf("/assets/detail?uuid=%s&message=%s",
		log.Asset.AssetUUID, url.QueryEscape("asset marked as returned")))
}

// HandoverSignGet renders the public digital signature form.
// GET /handover/sign?token=...
func (a *App) HandoverSignGet(c fiber.Ctx) error {
	token := c.Query("token")
	if token == "" {
		return c.Status(fiber.StatusBadRequest).SendString("invalid or missing token")
	}

	var form models.HandoverForm
	if err := a.DB.
		Preload("LendingLog").
		Preload("LendingLog.Asset").
		Preload("LendingLog.Assignee").
		Where("form_token = ?", token).
		First(&form).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("form not found or already completed")
	}

	if form.Status == "published" {
		return c.Render("handover_already_signed", fiber.Map{
			"Title": "Already Signed",
		})
	}

	return c.Render("handover_sign", fiber.Map{
		"Title": "Asset Handover Form",
		"Form":  form,
		"Token": token,
	})
}

// HandoverSignPost processes the submitted signature.
// POST /handover/sign
func (a *App) HandoverSignPost(c fiber.Ctx) error {
	token := strings.TrimSpace(c.FormValue("token"))
	signatureData := strings.TrimSpace(c.FormValue("signature_data")) // base64 PNG from canvas

	if token == "" || signatureData == "" {
		return c.Redirect().To("/handover/sign?token=" + url.QueryEscape(token) +
			"&error=" + url.QueryEscape("signature is required"))
	}

	var form models.HandoverForm
	if err := a.DB.
		Preload("LendingLog").
		Preload("LendingLog.Asset").
		Preload("LendingLog.Asset.Category").
		Preload("LendingLog.Assignee").
		Where("form_token = ?", token).
		First(&form).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("form not found")
	}

	if form.Status == "published" {
		return c.Redirect().To("/handover/sign?token=" + url.QueryEscape(token))
	}

	now := time.Now()
	a.DB.Model(&form).Updates(map[string]any{
		"signature_data": signatureData,
		"signed_at":      &now,
		"status":         "signed",
	})

	// Also update the lending log status.
	a.DB.Model(&models.LendingLog{}).Where("id = ?", form.LendingLogNo).Update("status", "active")

	receiptPath, err := utils.GenerateHandoverReceipt(utils.ReceiptData{
		AssetName:     form.LendingLog.Asset.Name,
		AssetType:     form.LendingLog.Asset.AssetType,
		SerialNumber:  form.LendingLog.Asset.SerialNumber,
		Category:      form.LendingLog.Asset.Category.Name,
		AssigneeName:  form.LendingLog.Assignee.FullName,
		AssigneeEmail: form.LendingLog.Assignee.Email,
		AssigneePhone: form.LendingLog.Assignee.PhoneNumber,
		LentAt:        form.LendingLog.LentAt,
		SignedAt:      now,
		SignatureData: signatureData,
	}, form.FormUUID)

	if err == nil {
		a.DB.Model(&form).Updates(map[string]any{
			"receipt_path": receiptPath,
			"status":       "published",
		})
		_ = a.sendHandoverReceiptEmail(
			form.LendingLog.Assignee.Email,
			form.LendingLog.Assignee.FullName,
			form.LendingLog.Asset.Name,
			receiptPath,
		)
	}

	return c.Render("handover_success", fiber.Map{
		"Title":    "Signed Successfully",
		"Assignee": form.LendingLog.Assignee,
		"Asset":    form.LendingLog.Asset,
	})
}

// HandoverReceiptDownload serves the generated receipt PDF.
// GET /handover/receipt?form_uuid=...
func (a *App) HandoverReceiptDownload(c fiber.Ctx) error {
	formUUID := c.Query("form_uuid")

	var form models.HandoverForm
	if err := a.DB.Where("form_uuid = ?", formUUID).First(&form).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("receipt not found")
	}

	if form.ReceiptPath == "" {
		return c.Status(fiber.StatusNotFound).SendString("receipt not yet generated")
	}

	return c.Download(form.ReceiptPath, "Handover_Receipt.pdf")
}
