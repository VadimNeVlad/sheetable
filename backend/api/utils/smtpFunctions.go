package utils

import (
	"fmt"
	"time"

	"github.com/SheetAble/SheetAble/backend/api/config"
	gomail "gopkg.in/mail.v2"
)

const smtpConnectionTimeout = 10 * time.Second

func SendPasswordResetEmail(resetPasswordId string, emailAdress string) error {
	if config.Config().Smtp.Enabled == "0" {
		return fmt.Errorf("SMTP is disabled")
	}
	m := gomail.NewMessage()

	// Set E-Mail sender
	m.SetHeader("From", config.Config().Smtp.From)

	// Set E-Mail receivers
	m.SetHeader("To", emailAdress)

	// Set E-Mail subject
	m.SetHeader("Subject", "Password Reset Request")

	// Set E-Mail body. You can set plain text or html with text/html
	m.SetBody("text/plain", "Hey there was a password reset request to your accout. Go to "+config.Config().ServerUrl+"/reset-password/"+resetPasswordId+" to update your password") // TODO: Make HTML + make resetPasswordId a frontend URL

	// Settings for SMTP server
	d := gomail.NewDialer(config.Config().Smtp.HostServerAddr,
		config.Config().Smtp.HostServerPort,
		config.Config().Smtp.Username,
		config.Config().Smtp.Password,
	)
	d.Timeout = smtpConnectionTimeout

	// Now send E-Mail
	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}

	fmt.Println("Sent password reset request email to: " + emailAdress)

	return nil
}
