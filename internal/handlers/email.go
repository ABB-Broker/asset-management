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
