package mailer

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/sli/backend/internal/config"
)

// Mailer sends an HTML email. Implementations must be safe to call from
// multiple goroutines (the outbox dispatcher and any direct/synchronous
// send path may both use the same instance concurrently).
type Mailer interface {
	Send(to []string, subject, htmlBody string) error
}

type smtpMailer struct {
	host     string
	port     int
	username string
	password string
	from     string
	fromName string
}

// NewSMTPMailer builds a Mailer that talks to a real SMTP server using
// STARTTLS (the standard submission-port pattern - port 587). Works with
// Gmail SMTP, SendGrid, Mailgun, SES, Mailtrap, or any other SMTP_HOST you
// point it at via env vars; see internal/config/config.go for the SMTP_*
// settings.
func NewSMTPMailer(cfg *config.Config) Mailer {
	return &smtpMailer{
		host:     cfg.SMTPHost,
		port:     cfg.SMTPPort,
		username: cfg.SMTPUsername,
		password: cfg.SMTPPassword,
		from:     cfg.SMTPFromAddress,
		fromName: cfg.SMTPFromName,
	}
}

func (m *smtpMailer) Send(to []string, subject, htmlBody string) error {
	if m.host == "" {
		return fmt.Errorf("SMTP is not configured (SMTP_HOST is empty) - cannot send %q", subject)
	}
	if len(to) == 0 {
		return fmt.Errorf("no recipients given for %q", subject)
	}

	addr := fmt.Sprintf("%s:%d", m.host, m.port)
	auth := smtp.PlainAuth("", m.username, m.password, m.host)

	msg := buildMessage(m.fromName, m.from, to, subject, htmlBody)

	// Port 465 is implicit TLS (the connection is TLS from the first byte);
	// every other port (587, 25, ...) is submitted in the clear and then
	// upgraded via STARTTLS, which smtp.SendMail handles internally as long
	// as the server advertises STARTTLS.
	if m.port == 465 {
		return sendImplicitTLS(addr, m.host, auth, m.from, to, msg)
	}
	return smtp.SendMail(addr, auth, m.from, to, msg)
}

func buildMessage(fromName, from string, to []string, subject, htmlBody string) []byte {
	var b strings.Builder
	if fromName != "" {
		fmt.Fprintf(&b, "From: %s <%s>\r\n", fromName, from)
	} else {
		fmt.Fprintf(&b, "From: %s\r\n", from)
	}
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(htmlBody)
	return []byte(b.String())
}

// sendImplicitTLS handles SMTPS (port 465), which smtp.SendMail doesn't
// support directly since it always dials in the clear first.
func sendImplicitTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}
