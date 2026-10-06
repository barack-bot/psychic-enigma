package export

import (
	"fmt"
	"net/smtp"
	"strings"
)

// Mailer sends plain-text email using SMTP.
type Mailer struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

func NewMailer(host string, port int, username, password, from string) *Mailer {
	return &Mailer{Host: host, Port: port, Username: username, Password: password, From: from}
}

// Send delivers a plain-text email to one or more recipients.
func (m *Mailer) Send(to []string, subject, body string) error {
	if m.Host == "" || len(to) == 0 {
		return fmt.Errorf("mailer not configured or no recipients")
	}
	addr := fmt.Sprintf("%s:%d", m.Host, m.Port)
	var auth smtp.Auth
	if m.Username != "" {
		auth = smtp.PlainAuth("", m.Username, m.Password, m.Host)
	}
	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		m.From, strings.Join(to, ","), subject, body)
	return smtp.SendMail(addr, auth, m.From, to, []byte(message))
}