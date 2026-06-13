package doctor

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// NotifyConfig holds the SMTP + addressing config needed to send a report email.
// Fields match the daemon's services.yaml `notify:` section.
type NotifyConfig struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	From         string
	To           string
	Hostname     string
}

// yamlNotify is used to parse the relevant subset of services.yaml.
type yamlNotify struct {
	Notify struct {
		SMTP struct {
			Host     string `yaml:"host"`
			Port     int    `yaml:"port"`
			Username string `yaml:"username"`
			Password string `yaml:"password"`
		} `yaml:"smtp"`
		From string `yaml:"from"`
		To   string `yaml:"to"`
	} `yaml:"notify"`
}

// LoadNotifyConfigFromFile reads SMTP config from the daemon's services.yaml.
// Returns a zero-value NotifyConfig (with empty SMTPHost) if parsing fails.
func LoadNotifyConfigFromFile(path string) (NotifyConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return NotifyConfig{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var y yamlNotify
	if err := yaml.Unmarshal(data, &y); err != nil {
		return NotifyConfig{}, fmt.Errorf("parse config: %w", err)
	}
	hostname, _ := os.Hostname()
	return NotifyConfig{
		SMTPHost:     y.Notify.SMTP.Host,
		SMTPPort:     y.Notify.SMTP.Port,
		SMTPUser:     y.Notify.SMTP.Username,
		SMTPPassword: y.Notify.SMTP.Password,
		From:         y.Notify.From,
		To:           y.Notify.To,
		Hostname:     hostname,
	}, nil
}

// Notify sends an SMTP email if the report contains failures and SMTP is configured.
// Returns nil silently if SMTPHost is empty (notifier disabled).
func Notify(report Report, cfg NotifyConfig) error {
	if report.Failed == 0 || cfg.SMTPHost == "" {
		return nil
	}
	subject := fmt.Sprintf("[homelab] Post-activation doctor: %d check(s) failed on %s", report.Failed, cfg.Hostname)
	body := FormatReportText(report)
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		cfg.From, cfg.To, subject, body,
	)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
	return smtp.SendMail(addr, auth, cfg.From, strings.Split(cfg.To, ","), []byte(msg))
}

// FormatReportText renders a Report as a human-readable plain-text string.
func FormatReportText(r Report) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Homelab Doctor Report — %d passed, %d failed\n", r.Passed, r.Failed))
	b.WriteString(strings.Repeat("=", 52) + "\n")
	for _, res := range r.Results {
		if res.OK {
			b.WriteString(fmt.Sprintf(" [✔] %s\n", res.Name))
			if res.Detail != "" {
				b.WriteString(fmt.Sprintf("     %s\n", res.Detail))
			}
		} else {
			b.WriteString(fmt.Sprintf(" [✗] %s\n", res.Name))
			if res.Detail != "" {
				b.WriteString(fmt.Sprintf("     Detail: %s\n", res.Detail))
			}
			if res.Fix != "" {
				b.WriteString(fmt.Sprintf("     Fix: %s\n", res.Fix))
			}
		}
	}
	return b.String()
}
