package services

import (
	"crypto/tls"
	"fmt"
	"html"
	"log"
	"net"
	"net/smtp"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// SendEmail sends an HTML email via SMTP (e.g. Gmail SMTP)
func SendEmail(toEmail, subject, htmlBody string) error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPortStr := os.Getenv("SMTP_PORT")
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	senderEmail := os.Getenv("SMTP_SENDER_EMAIL")
	senderName := os.Getenv("SMTP_SENDER_NAME")

	if smtpHost == "" || username == "" || password == "" {
		log.Println("[EmailService] SMTP credentials missing, skipping real email dispatch.")
		return fmt.Errorf("SMTP credentials not configured")
	}

	if senderEmail == "" {
		senderEmail = username
	}
	if senderName == "" {
		senderName = "PROPHIT Team"
	}
	if smtpPortStr == "" {
		smtpPortStr = "587"
	}
	smtpPort, _ := strconv.Atoi(smtpPortStr)

	// Construct MIME Headers & HTML Body
	header := make(map[string]string)
	header["From"] = fmt.Sprintf("%s <%s>", senderName, senderEmail)
	header["To"] = toEmail
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/html; charset=\"UTF-8\""

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + htmlBody

	auth := smtp.PlainAuth("", username, password, smtpHost)
	addr := net.JoinHostPort(smtpHost, strconv.Itoa(smtpPort))

	// Bound connection setup and the complete SMTP conversation.
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	var err error
	tlsConfig := &tls.Config{ServerName: smtpHost, MinVersion: tls.VersionTLS12}
	if smtpPort == 465 {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("SMTP connection failed: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, smtpHost)
	if err != nil {
		return err
	}
	defer client.Close()
	if smtpPort != 465 {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("SMTP server must support STARTTLS")
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return err
		}
	}
	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(senderEmail); err != nil {
		return err
	}
	if err := client.Rcpt(toEmail); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write([]byte(message)); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := client.Quit(); err != nil {
		return err
	}

	log.Printf("[EmailService] Successfully sent email to %s (Subject: %s)\n", toEmail, subject)
	return nil
}

// SendPasswordResetEmail sends a formatted password reset link email
func SendPasswordResetEmail(toEmail, username, resetToken string) error {
	appURL := strings.TrimRight(os.Getenv("APP_URL"), "/")
	if appURL == "" {
		appURL = "https://profhit.vercel.app"
	}
	resetURL := appURL + "/reset-password.html?token=" + url.QueryEscape(resetToken)
	subject := "Reset Your PROPHIT Password"
	htmlBody := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head><style>
		body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background-color: #0f172a; color: #e2e8f0; padding: 20px; }
		.card { background-color: #1e293b; border-radius: 12px; border: 1px solid #334155; max-width: 500px; margin: 0 auto; padding: 32px; text-align: center; }
		.title { color: #38bdf8; font-size: 24px; font-weight: 700; margin-bottom: 16px; }
		.btn { display: inline-block; background: linear-gradient(135deg, #6366f1 0%%, #a855f7 100%%); color: #ffffff !important; padding: 14px 28px; border-radius: 8px; text-decoration: none; font-weight: 600; margin: 24px 0; }
		.footer { font-size: 12px; color: #94a3b8; margin-top: 24px; }
	</style></head>
	<body>
		<div class="card">
			<div class="title">PROPHIT Password Reset</div>
			<p>Hello <strong>%s</strong>,</p>
			<p>We received a request to reset your PROPHIT account password. Click the button below to choose a new password:</p>
			<a href="%s" class="btn">Reset Password</a>
			<p style="font-size: 13px; color: #94a3b8;">If you did not request a password reset, you can safely ignore this email.</p>
			<div class="footer">&copy; PROPHIT Team. All rights reserved.</div>
		</div>
	</body>
	</html>
	`, html.EscapeString(username), html.EscapeString(resetURL))

	return SendEmail(toEmail, subject, htmlBody)
}
