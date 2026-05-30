package email

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/milzam/go-starter/internal/config"
)

// Sender defines email sending operations used by background tasks.
type Sender interface {
	Send(ctx context.Context, to, subject, body string) error
}

// SMTPSender sends plain text email through an SMTP server.
type SMTPSender struct {
	cfg config.EmailConfig
}

func NewSMTPSender(cfg config.EmailConfig) *SMTPSender {
	return &SMTPSender{cfg: cfg}
}

func (s *SMTPSender) Send(_ context.Context, to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	msg := strings.Join([]string{
		"From: " + s.cfg.From,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}
	return smtp.SendMail(addr, auth, s.cfg.From, []string{to}, []byte(msg))
}
