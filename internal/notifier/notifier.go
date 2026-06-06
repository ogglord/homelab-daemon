// Package notifier sends SMTP email alerts for daemon events.
package notifier

import (
	"fmt"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// SMTPConfig holds the connection details for the SMTP relay.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
}

// Notifier sends rate-limited email alerts.
type Notifier struct {
	smtp       SMTPConfig
	from       string
	to         string
	hostname   string
	mu         sync.Mutex
	lastNotify map[string]time.Time // notification type -> last sent time
}

// New creates a Notifier. If smtp.Host is empty, the notifier is a no-op.
func New(smtpCfg SMTPConfig, from, to, hostname string) *Notifier {
	return &Notifier{
		smtp:       smtpCfg,
		from:       from,
		to:         to,
		hostname:   hostname,
		lastNotify: make(map[string]time.Time),
	}
}

// Enabled returns true if SMTP is configured.
func (n *Notifier) Enabled() bool {
	return n.smtp.Host != ""
}

// Send sends an email immediately. Returns error if SMTP is not configured.
func (n *Notifier) Send(subject, body string) error {
	if !n.Enabled() {
		return fmt.Errorf("notifier: SMTP not configured")
	}
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		n.from, n.to, subject, body)

	addr := fmt.Sprintf("%s:%d", n.smtp.Host, n.smtp.Port)
	auth := smtp.PlainAuth("", n.smtp.Username, n.smtp.Password, n.smtp.Host)
	return smtp.SendMail(addr, auth, n.from, strings.Split(n.to, ","), []byte(msg))
}

// SendWithCooldown sends an email but only if the cooldown period has elapsed
// since the last send for the given key. If rate-limited, no error is returned.
func (n *Notifier) SendWithCooldown(key string, cooldown time.Duration, subject, body string) error {
	if !n.Enabled() {
		return nil
	}
	n.mu.Lock()
	last, ok := n.lastNotify[key]
	now := time.Now()
	if ok && now.Before(last.Add(cooldown)) {
		n.mu.Unlock()
		return nil // rate-limited, skip silently
	}
	n.lastNotify[key] = now
	n.mu.Unlock()

	return n.Send(subject, body)
}

// ── Pre-built notification helpers ──────────────────────────────────────

// BackupFailed returns a subject/body pair for a failed backup.
func (n *Notifier) BackupFailed(unit string, err error) (string, string) {
	subject := fmt.Sprintf("[%s] Backup failed: %s", n.hostname, unit)
	body := fmt.Sprintf("Backup job %s on %s failed.\n\nError: %s", unit, n.hostname, err.Error())
	return subject, body
}

// ServiceFailed returns a subject/body pair for a service failure.
func (n *Notifier) ServiceFailed(unit, name, reason string) (string, string) {
	subject := fmt.Sprintf("[%s] Service failure: %s", n.hostname, name)
	body := fmt.Sprintf("Service %s (%s) on %s is failing.\n\nStatus: %s", name, unit, n.hostname, reason)
	return subject, body
}

// DaemonCrash returns a subject/body pair for a detected crash.
func (n *Notifier) DaemonCrash() (string, string) {
	subject := fmt.Sprintf("[%s] Daemon recovered from crash", n.hostname)
	body := fmt.Sprintf("The homelab-daemon on %s was restarted after an unclean shutdown (possible crash).\n\nAll services are being re-monitored.", n.hostname)
	return subject, body
}
