package handlers

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"

	"github.com/ABB-Broker/asset-management/internal/config"
	"gopkg.in/gomail.v2"
)

//go:embed email-templates/*.html
var emailTemplates embed.FS

type EmailService struct{}

type PermissionStruct struct {
	ActionPage        string
	ActionName        string
	ActionDescription string
}

type OTPEmailData struct {
	Username string
	Code     string
}

type HandoverEmailData struct {
	Greeting  string
	AssetName string
	FormURL   string
}

type SetPasswordEmailData struct {
	Heading    string
	Greeting   string
	Body       template.HTML
	Link       string
	ButtonText string
}

type ReceiptEmailData struct {
	Greeting  string
	AssetName string
}

type ApprovalRequestEmailData struct {
	Greeting    string
	Borrower    string
	AssetName   string
	ApprovalURL string
}

func NewEmailService() *EmailService {
	return &EmailService{}
}

func renderEmailTemplate(templateName string, data any) (string, error) {
	templatePath := "email-templates/" + templateName

	tmpl, err := template.ParseFS(emailTemplates, templatePath)
	if err != nil {
		return "", err
	}

	var body bytes.Buffer

	if err := tmpl.Execute(&body, data); err != nil {
		return "", err
	}

	return body.String(), nil
}

func emailSender(body string, subject string, toEmail string) error {
	m := gomail.NewMessage()

	m.SetHeader(
		"From",
		fmt.Sprintf(
			"%s <%s>",
			config.EmailData.SMTP.Name,
			config.EmailData.SMTP.User,
		),
	)

	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(
		config.EmailData.SMTP.Server,
		int(config.EmailData.SMTPPort),
		config.EmailData.SMTP.User,
		config.EmailData.SMTP.Pass,
	)

	return d.DialAndSend(m)
}

func (a *App) sendOTPEmail(toEmail, username, code string) error {
	body, err := renderEmailTemplate(
		"otp.html",
		OTPEmailData{
			Username: username,
			Code:     code,
		},
	)

	if err != nil {
		return err
	}

	return emailSender(
		body,
		"Your Login Verification Code",
		toEmail,
	)
}

// sendHandoverEmail sends the digital signature form link to the assignee.
func (a *App) sendHandoverEmail(
	toEmail,
	assigneeName,
	assetName,
	formURL string,
) error {

	if toEmail == "" {
		return nil
	}

	greeting := "Hello,"

	if assigneeName != "" {
		greeting = "Hello, " + assigneeName + ","
	}

	body, err := renderEmailTemplate(
		"handover.html",
		HandoverEmailData{
			Greeting:  greeting,
			AssetName: assetName,
			FormURL:   formURL,
		},
	)

	if err != nil {
		return err
	}

	return emailSender(
		body,
		"Asset Handover — Signature Required",
		toEmail,
	)
}

func (a *App) sendSetPasswordEmail(
	toEmail,
	name,
	link,
	kind string,
) error {

	subject := "Welcome — Set Your Password"
	heading := "You've been invited to the Asset Management System"

	bodyText := `You have been invited to join the <strong>Information Systems Asset Management</strong> system.<br><br>
Click the button below to set your password. This link is valid for <strong>24 hours</strong>.`

	buttonText := "Set Password"

	if kind == "reset" {
		subject = "Reset Your Password"

		heading = "Password Reset Request"

		bodyText = `A password reset was requested for your account.<br><br>
Click the button below to set a new password. This link is valid for <strong>24 hours</strong>.<br><br>
If you did not request this, you can safely ignore this email.`

		buttonText = "Reset Password"
	}

	greeting := "Hello,"

	if name != "" {
		greeting = "Hello, " + name + ","
	}

	body, err := renderEmailTemplate(
		"set_password.html",
		SetPasswordEmailData{
			Heading:    heading,
			Greeting:   greeting,
			Body:       template.HTML(bodyText),
			Link:       link,
			ButtonText: buttonText,
		},
	)

	if err != nil {
		return err
	}

	return emailSender(
		body,
		subject,
		toEmail,
	)
}

// sendHandoverReceiptEmail sends the generated PDF receipt as an attachment to the assignee.
func (a *App) sendHandoverReceiptEmail(
	toEmail,
	assigneeName,
	assetName,
	receiptPath string,
) error {

	if toEmail == "" {
		return nil
	}

	greeting := "Hello,"

	if assigneeName != "" {
		greeting = "Hello, " + assigneeName + ","
	}

	body, err := renderEmailTemplate(
		"receipt.html",
		ReceiptEmailData{
			Greeting:  greeting,
			AssetName: assetName,
		},
	)

	if err != nil {
		return err
	}

	m := gomail.NewMessage()

	m.SetHeader(
		"From",
		fmt.Sprintf(
			"%s <%s>",
			config.EmailData.SMTP.Name,
			config.EmailData.SMTP.User,
		),
	)

	m.SetHeader("To", toEmail)

	m.SetHeader(
		"Subject",
		"Asset Handover Receipt — "+assetName,
	)

	m.SetBody("text/html", body)

	m.Attach(
		receiptPath,
		gomail.SetHeader(map[string][]string{
			"Content-Disposition": {
				`attachment; filename="Handover_Receipt.pdf"`,
			},
		}),
	)

	d := gomail.NewDialer(
		config.EmailData.SMTP.Server,
		int(config.EmailData.SMTPPort),
		config.EmailData.SMTP.User,
		config.EmailData.SMTP.Pass,
	)

	return d.DialAndSend(m)
}

// sendApprovalRequestEmail notifies the designated PIC that they need to approve
// a borrow request.
func (a *App) sendApprovalRequestEmail(
	toEmail,
	approverName,
	borrowerName,
	assetName,
	approvalURL string,
) error {

	if toEmail == "" {
		return nil
	}

	greeting := "Hello,"

	if approverName != "" {
		greeting = "Hello, " + approverName + ","
	}

	body, err := renderEmailTemplate(
		"approval_request.html",
		ApprovalRequestEmailData{
			Greeting:    greeting,
			Borrower:    borrowerName,
			AssetName:   assetName,
			ApprovalURL: approvalURL,
		},
	)

	if err != nil {
		return err
	}

	return emailSender(
		body,
		"Action Required: Approve Asset Borrow Request — "+assetName,
		toEmail,
	)
}
