//go:build !email

package config

type SMTPStruct struct {
	User   string
	Name   string
	Pass   string
	Server string
}

type EmailConfig struct {
	Protocol string
	SMTPPort uint
	SMTP     SMTPStruct
	Mailtype string
	Charset  string
	WordWrap bool
	NewLine  string
}

var EmailData = EmailConfig{}
