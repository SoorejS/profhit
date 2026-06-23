package services

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
)

// SendEmail sends a simple text email using standard SMTP.
func SendEmail(to string, subject string, body string) error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")

	if smtpHost == "" {
		// Mock behavior if no SMTP configured
		log.Printf("[MOCK EMAIL] To: %s | Subject: %s | Body: %s\n", to, subject, body)
		return nil
	}

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)

	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", to, subject, body))

	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, smtpUser, []string{to}, msg)
	if err != nil {
		log.Println("Error sending email:", err)
		return err
	}
	return nil
}
