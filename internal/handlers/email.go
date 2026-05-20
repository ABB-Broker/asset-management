package handlers

import (
	"bytes"
	"fmt"

	"github.com/ABB-Broker/asset-management/internal/config"
	"gopkg.in/gomail.v2"
)

type EmailService struct{}
type PermissionStruct struct {
	ActionPage        string
	ActionName        string
	ActionDescription string
}

func NewEmailService() *EmailService {
	return &EmailService{}
}

func emailSender(body bytes.Buffer, subject string, toEmail string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("%s <%s>", config.EmailData.SMTP.Name, config.EmailData.SMTP.User))
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body.String())

	d := gomail.NewDialer(
		config.EmailData.SMTP.Server,
		int(config.EmailData.SMTPPort),
		config.EmailData.SMTP.User,
		config.EmailData.SMTP.Pass,
	)

	return d.DialAndSend(m)
}

func (a *App) sendOTPEmail(toEmail, username, code string) error {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, `<!DOCTYPE html>
<html><body style="font-family:sans-serif;background:#f1f5f9;margin:0;padding:32px;">
<div style="max-width:480px;margin:auto;background:#fff;border-radius:10px;overflow:hidden;box-shadow:0 2px 12px rgba(0,0,0,.08);">
  <div style="background:#0f172a;padding:24px 32px;">
    <h2 style="color:#fff;margin:0;font-size:1.15rem;">Two-Factor Verification Code</h2>
  </div>
  <div style="padding:28px 32px;text-align:center;">
    <p style="margin-top:0;color:#334155;">Hello <strong>%s</strong>,</p>
    <p style="color:#334155;">Your one-time login code for the Asset Management System is:</p>
    <div style="background:#f0f9ff;border:2px dashed #bae6fd;border-radius:10px;padding:20px 0;margin:24px 0;">
      <span style="font-size:2.4rem;font-weight:800;letter-spacing:.35em;color:#0f172a;">%s</span>
    </div>
    <p style="color:#64748b;font-size:.88rem;">This code expires in <strong>10 minutes</strong>. Do not share it with anyone.</p>
  </div>
  <div style="background:#f8fafc;padding:14px 32px;border-top:1px solid #e2e8f0;">
    <p style="color:#94a3b8;font-size:.78rem;margin:0;">Information Systems Asset Management &copy; 2026</p>
  </div>
</div></body></html>`, username, code)

	return emailSender(buf, "Your Login Verification Code", toEmail)
}

// sendHandoverEmail sends the digital signature form link to the assignee.
func (a *App) sendHandoverEmail(toEmail, assigneeName, assetName, formURL string) error {
	if toEmail == "" {
		return nil
	}
	greeting := "Hello,"
	if assigneeName != "" {
		greeting = "Hello, " + assigneeName + ","
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, `<!DOCTYPE html>
<html><body style="font-family:sans-serif;background:#f1f5f9;margin:0;padding:32px;">
<div style="max-width:520px;margin:auto;background:#fff;border-radius:10px;overflow:hidden;box-shadow:0 2px 12px rgba(0,0,0,.08);">
  <div style="background:#0f172a;padding:28px 32px;">
    <h2 style="color:#fff;margin:0;font-size:1.2rem;">Asset Handover Form</h2>
  </div>
  <div style="padding:28px 32px;">
    <p style="margin-top:0;">%s</p>
    <p>An asset has been assigned to you and requires your digital signature to complete the handover process.</p>
    <div style="background:#f8fafc;border:1px solid #e2e8f0;border-radius:8px;padding:16px 20px;margin:20px 0;">
      <strong style="color:#0f172a;">Asset:</strong> <span style="color:#334155;">%s</span>
    </div>
    <p>Please click the button below to review the handover details and sign digitally.</p>
    <div style="text-align:center;margin:28px 0;">
      <a href="%s" style="background:#2563eb;color:#fff;padding:12px 28px;border-radius:6px;text-decoration:none;font-weight:600;display:inline-block;">Sign Handover Form</a>
    </div>
    <p style="color:#64748b;font-size:.85rem;">Or copy this link into your browser:<br><a href="%s" style="color:#2563eb;">%s</a></p>
  </div>
  <div style="background:#f8fafc;padding:16px 32px;border-top:1px solid #e2e8f0;">
    <p style="color:#94a3b8;font-size:.78rem;margin:0;">Information Systems Asset Management &copy; 2026</p>
  </div>
</div></body></html>`, greeting, assetName, formURL, formURL, formURL)

	return emailSender(buf, "Asset Handover — Signature Required", toEmail)
}

func (a *App) sendSetPasswordEmail(toEmail, name, link, kind string) error {
	subject := "Welcome — Set Your Password"
	heading := "You've been invited to the Asset Management System"
	body := `You have been invited to join the <strong>Information Systems Asset Management</strong> system.<br><br>
Click the button below to set your password. This link is valid for <strong>24 hours</strong>.`

	if kind == "reset" {
		subject = "Reset Your Password"
		heading = "Password Reset Request"
		body = `A password reset was requested for your account.<br><br>
Click the button below to set a new password. This link is valid for <strong>24 hours</strong>.<br><br>
If you did not request this, you can safely ignore this email.`
	}

	greeting := "Hello,"
	if name != "" {
		greeting = "Hello, " + name + ","
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, `<!DOCTYPE html>
<html><body style="font-family:sans-serif;background:#f1f5f9;margin:0;padding:32px;">
<div style="max-width:520px;margin:auto;background:#fff;border-radius:10px;overflow:hidden;box-shadow:0 2px 12px rgba(0,0,0,.08);">
  <div style="background:#0f172a;padding:28px 32px;">
    <h2 style="color:#fff;margin:0;font-size:1.2rem;">%s</h2>
  </div>
  <div style="padding:28px 32px;">
    <p style="margin-top:0;">%s</p>
    <p>%s</p>
    <div style="text-align:center;margin:28px 0;">
      <a href="%s" style="background:#2563eb;color:#fff;padding:12px 28px;border-radius:6px;text-decoration:none;font-weight:600;display:inline-block;">Set Password</a>
    </div>
    <p style="color:#64748b;font-size:.85rem;">Or copy this link:<br><a href="%s" style="color:#2563eb;">%s</a></p>
  </div>
  <div style="background:#f8fafc;padding:16px 32px;border-top:1px solid #e2e8f0;">
    <p style="color:#94a3b8;font-size:.78rem;margin:0;">Information Systems Asset Management &copy; 2026</p>
  </div>
</div></body></html>`, heading, greeting, body, link, link, link)

	return emailSender(buf, subject, toEmail)
}

// sendHandoverReceiptEmail sends the generated PDF receipt as an attachment to the assignee.
func (a *App) sendHandoverReceiptEmail(toEmail, assigneeName, assetName, receiptPath string) error {
	if toEmail == "" {
		return nil
	}
	greeting := "Hello,"
	if assigneeName != "" {
		greeting = "Hello, " + assigneeName + ","
	}

	m := gomail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("%s <%s>", config.EmailData.SMTP.Name, config.EmailData.SMTP.User))
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", "Asset Handover Receipt — "+assetName)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, `<!DOCTYPE html>
<html><body style="font-family:sans-serif;background:#f1f5f9;margin:0;padding:32px;">
<div style="max-width:520px;margin:auto;background:#fff;border-radius:10px;overflow:hidden;box-shadow:0 2px 12px rgba(0,0,0,.08);">
  <div style="background:#0f172a;padding:28px 32px;">
    <h2 style="color:#fff;margin:0;font-size:1.2rem;">Asset Handover Receipt</h2>
  </div>
  <div style="padding:28px 32px;">
    <p style="margin-top:0;">%s</p>
    <p>Thank you for signing the asset handover form. Please find your signed receipt for <strong>%s</strong> attached to this email.</p>
    <p style="color:#64748b;font-size:.85rem;">Keep this receipt for your records. If you have any questions, please contact your system administrator.</p>
  </div>
  <div style="background:#f8fafc;padding:16px 32px;border-top:1px solid #e2e8f0;">
    <p style="color:#94a3b8;font-size:.78rem;margin:0;">Information Systems Asset Management &copy; 2026</p>
  </div>
</div></body></html>`, greeting, assetName)

	m.SetBody("text/html", buf.String())
	m.Attach(receiptPath, gomail.SetHeader(map[string][]string{
		"Content-Disposition": {`attachment; filename="Handover_Receipt.pdf"`},
	}))

	d := gomail.NewDialer(
		config.EmailData.SMTP.Server,
		int(config.EmailData.SMTPPort),
		config.EmailData.SMTP.User,
		config.EmailData.SMTP.Pass,
	)
	return d.DialAndSend(m)
}
