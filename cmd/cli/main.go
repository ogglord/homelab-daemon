package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ogglord/homelab-daemon/internal/doctor"
	"github.com/urfave/cli/v2"
)

const socketPath = "/run/homelab-daemon/daemon.sock"

var httpClient = &http.Client{
	Transport: &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	},
	Timeout: 60 * time.Second,
}

var (
	Version   = "0.3.0"
	BuildDate = "2026-06-13"
)

func main() {
	app := &cli.App{
		Name:    "homelab",
		Usage:   "Manage your homelab services, backups, secrets, and health",
		Version:              fmt.Sprintf("%s (built %s)", Version, BuildDate),
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "json",
				Usage: "output as JSON",
			},
		},
		Commands: []*cli.Command{
			servicesCmd(),
			backupCmd(),
			secretCmd(),
			configCmd(),
			daemonCmd(),
			doctorCmd(),
			notifyCmd(),
			updateCmd(),
			storageCmd(),
			vmCmd(),
			deployCmd(),
			completionCmd(),
			mergeConfigCmd(),
		},
		Action: func(c *cli.Context) error {
			if c.Args().First() == "" {
				return cli.ShowAppHelp(c)
			}
			return fmt.Errorf("unknown command %q — run 'homelab help'", c.Args().First())
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

func jsonFlag(c *cli.Context) bool {
	for _, ctx := range c.Lineage() {
		if ctx.Bool("json") {
			return true
		}
	}
	return false
}

// ── services ─────────────────────────────────────────────────────────────────

type serviceStatus struct {
	Unit          string `json:"unit"`
	Enabled       bool   `json:"enabled"`
	Active        bool   `json:"active"`
	UserStopped   bool   `json:"user_stopped"`
	Restart       string `json:"restart"`
	Order         int    `json:"order"`
	FailureCount  int    `json:"failure_count"`
	BackoffUntil  string `json:"backoff_until"`
	BlockedReason string `json:"blocked_reason"`
}

func servicesCmd() *cli.Command {
	return &cli.Command{
		Name:    "services",
		Aliases: []string{"s"},
		Usage:   "Manage homelab services",
		Subcommands: []*cli.Command{
			{
				Name:  "status",
				Usage: "List status of all managed services",
				Action: func(c *cli.Context) error {
					handleStatus(jsonFlag(c))
					return nil
				},
			},
			{
				Name:      "start",
				Usage:     "Start a service",
				ArgsUsage: "<unit>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: homelab services start <unit>")
					}
					handleStart(c.Args().First())
					return nil
				},
			},
			{
				Name:      "stop",
				Usage:     "Stop a service",
				ArgsUsage: "<unit>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: homelab services stop <unit>")
					}
					handleStop(c.Args().First())
					return nil
				},
			},
			{
				Name:      "restart",
				Usage:     "Restart a service",
				ArgsUsage: "<unit>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: homelab services restart <unit>")
					}
					handleRestart(c.Args().First())
					return nil
				},
			},
			{
				Name:      "enable",
				Usage:     "Enable a service (autostart)",
				ArgsUsage: "<unit>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: homelab services enable <unit>")
					}
					handleEnable(c.Args().First(), true)
					return nil
				},
			},
			{
				Name:      "disable",
				Usage:     "Disable a service",
				ArgsUsage: "<unit>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: homelab services disable <unit>")
					}
					handleEnable(c.Args().First(), false)
					return nil
				},
			},
			{
				Name:      "logs",
				Usage:     "Tail logs for a service",
				ArgsUsage: "<unit>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: homelab services logs <unit>")
					}
					handleLogs(c.Args().First())
					return nil
				},
			},
		},
	}
}

func handleStatus(asJSON bool) {
	resp, err := httpClient.Get("http://unix/api/status")
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		die("daemon returned status %d", resp.StatusCode)
	}
	var services []serviceStatus
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		die("decoding response: %v", err)
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(services)
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "UNIT\tACTIVE\tENABLED\tFAILURES\tBLOCKED")
	for _, s := range services {
		fmt.Fprintf(w, "%s\t%v\t%v\t%d\t%s\n", s.Unit, s.Active, s.Enabled, s.FailureCount, s.BlockedReason)
	}
	w.Flush()
}

func handleStart(unit string) {
	resp, err := httpClient.Post(fmt.Sprintf("http://unix/api/start/%s", unit), "application/json", nil)
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		die("daemon returned status %d", resp.StatusCode)
	}
	fmt.Printf("Started %s\n", unit)
}

func handleStop(unit string) {
	resp, err := httpClient.Post(fmt.Sprintf("http://unix/api/stop/%s", unit), "application/json", nil)
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		die("daemon returned status %d", resp.StatusCode)
	}
	fmt.Printf("Stopped %s\n", unit)
}

func handleRestart(unit string) {
	// Route through daemon socket (consistent with start/stop).
	resp, err := httpClient.Post(fmt.Sprintf("http://unix/api/restart/%s", unit), "application/json", nil)
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		die("daemon returned status %d", resp.StatusCode)
	}
	fmt.Printf("Restarted %s\n", unit)
}

func handleEnable(unit string, enable bool) {
	payload := map[string]bool{"enabled": enable}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("http://unix/api/config/%s", unit), bytes.NewReader(b))
	if err != nil {
		die("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		die("daemon returned status %d", resp.StatusCode)
	}
	action := "Disabled"
	if enable {
		action = "Enabled"
	}
	fmt.Printf("%s %s\n", action, unit)
}

func handleLogs(unit string) {
	cmd := exec.Command("journalctl", "-fu", unit)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		die("running journalctl: %v", err)
	}
}

// ── backup ───────────────────────────────────────────────────────────────────

func backupCmd() *cli.Command {
	return &cli.Command{
		Name:  "backup",
		Usage: "Manage backup jobs",
		Subcommands: []*cli.Command{
			{
				Name:  "status",
				Usage: "Show last-run times for all backup jobs",
				Action: func(c *cli.Context) error {
					handleBackupStatus(jsonFlag(c))
					return nil
				},
			},
			{
				Name:      "run",
				Usage:     "Trigger a backup job manually",
				ArgsUsage: "<unit>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: homelab backup run <unit>")
					}
					handleBackupRun(c.Args().First())
					return nil
				},
			},
		},
	}
}

func handleBackupStatus(asJSON bool) {
	resp, err := httpClient.Get("http://unix/api/backups")
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		die("daemon returned status %d", resp.StatusCode)
	}
	var result any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		die("decoding response: %v", err)
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return
	}
	b, _ := json.Marshal(result)
	var backups []struct {
		Unit    string `json:"unit"`
		LastRun string `json:"last_run"`
		Next    string `json:"next_run"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.Unmarshal(b, &backups); err != nil || len(backups) == 0 {
		fmt.Println(string(b))
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "UNIT\tENABLED\tLAST RUN\tNEXT RUN")
	for _, bk := range backups {
		fmt.Fprintf(w, "%s\t%v\t%s\t%s\n", bk.Unit, bk.Enabled, bk.LastRun, bk.Next)
	}
	w.Flush()
}

func handleBackupRun(unit string) {
	resp, err := httpClient.Post(fmt.Sprintf("http://unix/api/backups/%s/run", unit), "application/json", nil)
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		die("daemon returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	fmt.Printf("Triggered backup: %s\n", unit)
}

// ── secret ───────────────────────────────────────────────────────────────────

type secretEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Present     bool   `json:"present"`
	ModifiedAt  string `json:"modified_at"`
	Preview     string `json:"preview"`
}

func secretCmd() *cli.Command {
	return &cli.Command{
		Name:  "secret",
		Usage: "Manage homelab secrets",
		Before: func(c *cli.Context) error {
			sub := c.Args().First()
			if sub == "add" || sub == "set" {
				if os.Geteuid() != 0 {
					return fmt.Errorf("homelab secret %s requires sudo", sub)
				}
			}
			return nil
		},
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all declared secrets and their status",
				Action: func(c *cli.Context) error {
					handleSecretList(jsonFlag(c))
					return nil
				},
			},
			{
				Name:      "add",
				Usage:     "Declare and set a new secret",
				ArgsUsage: "<name>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: homelab secret add <name>")
					}
					handleSecretAdd(c.Args().First())
					return nil
				},
			},
			{
				Name:      "set",
				Usage:     "Rotate the value of an existing secret",
				ArgsUsage: "[name]",
				Action: func(c *cli.Context) error {
					handleSecretSet(c.Args().Slice())
					return nil
				},
			},
		},
	}
}

func handleSecretList(asJSON bool) {
	resp, err := httpClient.Get("http://unix/api/secrets")
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		die("daemon returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var result struct {
		Secrets       []secretEntry `json:"secrets"`
		DeployPending bool          `json:"deploy_pending"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		die("decoding response: %v", err)
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tDESCRIPTION\tPRESENT\tMODIFIED")
	for _, s := range result.Secrets {
		present := "✗"
		if s.Present {
			present = "✔"
		}
		modified := ""
		if s.ModifiedAt != "" {
			if t, err := time.Parse(time.RFC3339, s.ModifiedAt); err == nil {
				modified = t.Local().Format("2006-01-02 15:04")
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Name, s.Description, present, modified)
	}
	w.Flush()
	if result.DeployPending {
		fmt.Println("\n⚠ Deploy pending — run 'nh os switch' to apply secret changes.")
	}
}

func handleSecretAdd(name string) {
	fmt.Printf("Adding new secret: %s\n", name)
	desc, err := readLine("Description: ")
	if err != nil {
		die("reading description: %v", err)
	}
	val, err := readPassword(fmt.Sprintf("Value for %s: ", name))
	if err != nil {
		die("reading value: %v", err)
	}
	if val == "" {
		die("value must not be empty")
	}
	body := map[string]string{"value": val, "description": desc}
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://unix/api/secrets/%s", name), bytes.NewReader(b))
	if err != nil {
		die("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		die("daemon returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var result struct {
		Success       bool `json:"success"`
		DeployPending bool `json:"deploy_pending"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		die("decoding response: %v", err)
	}
	fmt.Printf("✓ Secret '%s' added.", name)
	if result.DeployPending {
		fmt.Println(" Deploy pending — run 'nh os switch' to apply.")
	} else {
		fmt.Println()
	}
}

func handleSecretSet(args []string) {
	if len(args) >= 1 && args[0] != "" {
		setSecretByName(args[0])
		return
	}
	name := pickSecretInteractive()
	if name == "" {
		return
	}
	setSecretByName(name)
}

func pickSecretInteractive() string {
	resp, err := httpClient.Get("http://unix/api/secrets")
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	var result struct {
		Secrets []secretEntry `json:"secrets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		die("decoding response: %v", err)
	}
	if len(result.Secrets) == 0 {
		fmt.Fprintln(os.Stderr, "No secrets registered.")
		return ""
	}
	fmt.Println("Select a secret to set:")
	for i, s := range result.Secrets {
		status := "✗"
		if s.Present {
			status = "✔"
		}
		modified := ""
		if s.ModifiedAt != "" {
			if t, err := time.Parse(time.RFC3339, s.ModifiedAt); err == nil {
				modified = " (last set: " + t.Local().Format("2006-01-02 15:04") + ")"
			}
		}
		fmt.Printf("  %2d. %s %s — %s%s\n", i+1, status, s.Name, s.Description, modified)
	}
	fmt.Println("   q. Quit")
	fmt.Print("\nChoice: ")
	var input string
	fmt.Scanln(&input)
	if input == "q" || input == "Q" || input == "" {
		return ""
	}
	n, err := strconv.Atoi(input)
	if err != nil || n < 1 || n > len(result.Secrets) {
		fmt.Fprintln(os.Stderr, "Invalid selection.")
		return ""
	}
	return result.Secrets[n-1].Name
}

func setSecretByName(name string) {
	val, err := readPassword(fmt.Sprintf("Value for %s: ", name))
	if err != nil {
		die("reading input: %v", err)
	}
	if val == "" {
		die("value must not be empty")
	}
	body := map[string]string{"value": val}
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://unix/api/secrets/%s", name), bytes.NewReader(b))
	if err != nil {
		die("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		die("daemon returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var result struct {
		Success       bool `json:"success"`
		DeployPending bool `json:"deploy_pending"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		die("decoding response: %v", err)
	}
	fmt.Printf("✓ Secret '%s' updated.", name)
	if result.DeployPending {
		fmt.Println(" Deploy pending — run 'nh os switch' to apply.")
	} else {
		fmt.Println()
	}
}

// ── config ───────────────────────────────────────────────────────────────────

func configCmd() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "Inspect daemon configuration",
		Subcommands: []*cli.Command{
			{
				Name:  "show",
				Usage: "Dump the current resolved configuration as JSON",
				Action: func(c *cli.Context) error {
					handleConfigShow()
					return nil
				},
			},
		},
	}
}

func handleConfigShow() {
	resp, err := httpClient.Get("http://unix/api/config")
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		die("daemon returned status %d", resp.StatusCode)
	}
	var result any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		die("decoding response: %v", err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
}

// ── daemon ───────────────────────────────────────────────────────────────────

func daemonCmd() *cli.Command {
	return &cli.Command{
		Name:  "daemon",
		Usage: "Inspect daemon process",
		Subcommands: []*cli.Command{
			{
				Name:  "status",
				Usage: "Lightweight ping to the daemon socket",
				Action: func(c *cli.Context) error {
					handleDaemonStatus(jsonFlag(c))
					return nil
				},
			},
		},
	}
}

func handleDaemonStatus(asJSON bool) {
	resp, err := httpClient.Get("http://unix/api/health")
	if err != nil {
		if asJSON {
			fmt.Println(`{"ok":false,"error":"daemon unreachable"}`)
		} else {
			fmt.Fprintln(os.Stderr, "✗ Daemon unreachable")
		}
		os.Exit(1)
	}
	defer resp.Body.Close()
	if asJSON {
		fmt.Println(`{"ok":true}`)
	} else {
		fmt.Println("✔ Daemon is running")
	}
}

// ── doctor ───────────────────────────────────────────────────────────────────

func doctorCmd() *cli.Command {
	return &cli.Command{
		Name:  "doctor",
		Usage: "Run diagnostic health checks",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "json",
				Usage: "output report as JSON (always exits 0 unless --fail-on-error)",
			},
			&cli.BoolFlag{
				Name:  "fail-on-error",
				Usage: "exit 1 if any check fails",
			},
			&cli.StringFlag{
				Name:  "check",
				Usage: "comma-separated list of check slugs to run (default: all)",
			},
		},
		Subcommands: []*cli.Command{
			{
				Name:  "notify",
				Usage: "Read a JSON doctor report from stdin and send SMTP if failures present",
				Action: func(c *cli.Context) error {
					return handleDoctorNotify()
				},
			},
		},
		Action: func(c *cli.Context) error {
			return handleDoctor(c)
		},
	}
}

func handleDoctor(c *cli.Context) error {
	asJSON := c.Bool("json") || jsonFlag(c)
	failOnError := c.Bool("fail-on-error")

	var slugs []string
	if raw := c.String("check"); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			if s = strings.TrimSpace(s); s != "" {
				slugs = append(slugs, s)
			}
		}
	}

	report := doctor.Run(slugs)

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		fmt.Println("🏥 Homelab Diagnostics")
		fmt.Println(strings.Repeat("=", 52))
		fmt.Print(doctor.FormatReportText(report))
		fmt.Println(strings.Repeat("=", 52))
		if report.Failed == 0 {
			fmt.Println("🎉 All checks passed.")
		} else {
			fmt.Printf("⚠️  %d check(s) failed.\n", report.Failed)
		}
	}

	if failOnError && report.Failed > 0 {
		os.Exit(1)
	}
	return nil
}

func handleDoctorNotify() error {
	var report doctor.Report
	if err := json.NewDecoder(os.Stdin).Decode(&report); err != nil {
		return fmt.Errorf("reading report from stdin: %w", err)
	}
	if report.Failed == 0 {
		return nil
	}
	const configPath = "/cache/appdata/homelab/services.yaml"
	cfg, err := doctor.LoadNotifyConfigFromFile(configPath)
	if err != nil {
		return fmt.Errorf("loading notify config: %w", err)
	}
	if err := doctor.Notify(report, cfg); err != nil {
		return fmt.Errorf("sending notification: %w", err)
	}
	fmt.Printf("Notification sent: %d check(s) failed\n", report.Failed)
	return nil
}

// ── notify ───────────────────────────────────────────────────────────────────

func notifyCmd() *cli.Command {
	return &cli.Command{
		Name:  "notify",
		Usage: "Test and manage notifications",
		Subcommands: []*cli.Command{
			{
				Name:  "test",
				Usage: "Send a test SMTP notification to verify email delivery",
				Action: func(c *cli.Context) error {
					return handleNotifyTest()
				},
			},
		},
	}
}

func handleNotifyTest() error {
	const configPath = "/cache/appdata/homelab/services.yaml"
	cfg, err := doctor.LoadNotifyConfigFromFile(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if cfg.SMTPHost == "" {
		return fmt.Errorf("SMTP not configured in %s", configPath)
	}
	return sendTestEmail(cfg)
}

func sendTestEmail(cfg doctor.NotifyConfig) error {
	hostname := cfg.Hostname
	if hostname == "" {
		hostname = "homelab"
	}
	subject := fmt.Sprintf("[homelab] Test notification from %s", hostname)
	body := fmt.Sprintf("This is a test notification sent from the homelab CLI on %s.\n\nIf you received this, SMTP is configured correctly.", hostname)
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		cfg.From, cfg.To, subject, body,
	)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	var auth smtp.Auth
	if cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
	}
	if err := smtp.SendMail(addr, auth, cfg.From, strings.Split(cfg.To, ","), []byte(msg)); err != nil {
		return fmt.Errorf("send failed: %w", err)
	}
	fmt.Printf("✔ Test email sent to %s\n", cfg.To)
	return nil
}

// ── updates ──────────────────────────────────────────────────────────────────

func updateCmd() *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "Check for container image updates",
		Subcommands: []*cli.Command{
			{
				Name:  "status",
				Usage: "Show available container image updates",
				Action: func(c *cli.Context) error {
					handleUpdateStatus(jsonFlag(c))
					return nil
				},
			},
			{
				Name:  "check",
				Usage: "Trigger a fresh update check",
				Action: func(c *cli.Context) error {
					return handleUpdateCheck()
				},
			},
		},
	}
}

func handleUpdateStatus(asJSON bool) {
	resp, err := httpClient.Get("http://unix/api/updates")
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		die("daemon returned status %d", resp.StatusCode)
	}
	var payload struct {
		Updates map[string]struct {
			HasUpdate      bool   `json:"has_update"`
			CurrentVersion string `json:"current_version"`
			RemoteVersion  string `json:"remote_version"`
		} `json:"updates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		die("decoding response: %v", err)
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(payload)
		return
	}
	if len(payload.Updates) == 0 {
		fmt.Println("No container update info available.")
		return
	}
	// Collect entries with updates first, then rest.
	type entry struct {
		name string
		u    struct {
			HasUpdate      bool   `json:"has_update"`
			CurrentVersion string `json:"current_version"`
			RemoteVersion  string `json:"remote_version"`
		}
	}
	var withUpdate, noUpdate []entry
	for name, u := range payload.Updates {
		e := entry{name: name, u: u}
		if u.HasUpdate {
			withUpdate = append(withUpdate, e)
		} else {
			noUpdate = append(noUpdate, e)
		}
	}
	sort.Slice(withUpdate, func(i, j int) bool { return withUpdate[i].name < withUpdate[j].name })
	sort.Slice(noUpdate, func(i, j int) bool { return noUpdate[i].name < noUpdate[j].name })
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "CONTAINER\tCURRENT\tNEWEST\tAVAILABLE")
	for _, e := range withUpdate {
		fmt.Fprintf(w, "%s\t%s\t%s\t✔\n", e.name, e.u.CurrentVersion, e.u.RemoteVersion)
	}
	for _, e := range noUpdate {
		fmt.Fprintf(w, "%s\t%s\t%s\t-\n", e.name, e.u.CurrentVersion, e.u.RemoteVersion)
	}
	w.Flush()
	if len(withUpdate) > 0 {
		fmt.Printf("\n%d update(s) available.\n", len(withUpdate))
	} else {
		fmt.Println("\nAll containers up to date.")
	}
}

func handleUpdateCheck() error {
	resp, err := httpClient.Post("http://unix/api/updates/check", "application/json", nil)
	if err != nil {
		return fmt.Errorf("contacting daemon: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon returned status %d", resp.StatusCode)
	}
	fmt.Println("✔ Update check triggered.")
	return nil
}

// ── storage ──────────────────────────────────────────────────────────────────

func storageCmd() *cli.Command {
	return &cli.Command{
		Name:  "storage",
		Usage: "Show bcachefs pool status",
		Subcommands: []*cli.Command{
			{
				Name:  "status",
				Usage: "Show status of all storage pools",
				Action: func(c *cli.Context) error {
					handleStorageStatus(jsonFlag(c))
					return nil
				},
			},
		},
	}
}

func handleStorageStatus(asJSON bool) {
	resp, err := httpClient.Get("http://unix/api/storage")
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		die("daemon returned status %d", resp.StatusCode)
	}
	var payload struct {
		Pools []struct {
			UUID     string `json:"uuid"`
			Name     string `json:"name"`
			State    string `json:"state"`
			Mountdir string `json:"mountdir"`
			Usage    *struct {
				TotalBytes  uint64  `json:"total_bytes"`
				UsedBytes   uint64  `json:"used_bytes"`
				UsedPercent float64 `json:"used_percent"`
			} `json:"usage"`
		} `json:"pools"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		die("decoding response: %v", err)
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(payload)
		return
	}
	if len(payload.Pools) == 0 {
		fmt.Println("No storage pools found.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tMOUNT\tSTATE\tUSED\tTOTAL\tUSED%")
	for _, p := range payload.Pools {
		name := p.Name
		if name == "" {
			if len(p.UUID) >= 8 {
				name = "pool-" + p.UUID[:8]
			} else {
				name = p.UUID
			}
		}
		used, total, pct := "-", "-", "-"
		if p.Usage != nil {
			used = fmtStorageBytes(p.Usage.UsedBytes)
			total = fmtStorageBytes(p.Usage.TotalBytes)
			pct = fmt.Sprintf("%.1f%%", p.Usage.UsedPercent)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", name, p.Mountdir, p.State, used, total, pct)
	}
	w.Flush()
}

func fmtStorageBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// ── vm ───────────────────────────────────────────────────────────────────────

type vmInfo struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Memory string `json:"memory"`
	CPUs   uint   `json:"cpus"`
}

func vmCmd() *cli.Command {
	return &cli.Command{
		Name:  "vm",
		Usage: "Manage virtual machines",
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all virtual machines and their state",
				Action: func(c *cli.Context) error {
					handleVMList(jsonFlag(c))
					return nil
				},
			},
			{
				Name:      "start",
				Usage:     "Start a VM",
				ArgsUsage: "<name>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: homelab vm start <name>")
					}
					return handleVMAction(c.Args().First(), "start")
				},
			},
			{
				Name:      "shutdown",
				Usage:     "Gracefully shut down a VM",
				ArgsUsage: "<name>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: homelab vm shutdown <name>")
					}
					return handleVMAction(c.Args().First(), "shutdown")
				},
			},
			{
				Name:      "suspend",
				Usage:     "Suspend (pause) a VM",
				ArgsUsage: "<name>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: homelab vm suspend <name>")
					}
					return handleVMAction(c.Args().First(), "suspend")
				},
			},
			{
				Name:      "resume",
				Usage:     "Resume a suspended VM",
				ArgsUsage: "<name>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: homelab vm resume <name>")
					}
					return handleVMAction(c.Args().First(), "resume")
				},
			},
		},
	}
}

func handleVMList(asJSON bool) {
	resp, err := httpClient.Get("http://unix/api/vms")
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		die("daemon returned status %d", resp.StatusCode)
	}
	var vms []vmInfo
	if err := json.NewDecoder(resp.Body).Decode(&vms); err != nil {
		die("decoding response: %v", err)
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(vms)
		return
	}
	if len(vms) == 0 {
		fmt.Println("No VMs found.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATE\tMEMORY\tCPUs")
	for _, v := range vms {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", v.Name, v.State, v.Memory, v.CPUs)
	}
	w.Flush()
}

func handleVMAction(name, action string) error {
	resp, err := httpClient.Post(
		fmt.Sprintf("http://unix/api/vms/%s/%s", name, action),
		"application/json", nil,
	)
	if err != nil {
		return fmt.Errorf("contacting daemon: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon returned status %d", resp.StatusCode)
	}
	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	if !result.Success {
		msg := result.Error
		if msg == "" {
			msg = "daemon reported failure (no message)"
		}
		return fmt.Errorf("VM action failed: %s", msg)
	}
	fmt.Printf("✔ VM %s: %s\n", name, action)
	return nil
}

// ── deploy ───────────────────────────────────────────────────────────────────

func deployCmd() *cli.Command {
	return &cli.Command{
		Name:  "deploy",
		Usage: "Update homelab-daemon flake input and switch NixOS configuration",
		Subcommands: []*cli.Command{
			{
				Name:  "switch",
				Usage: "Update homelab-daemon flake input and run nh os switch",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "no-update",
						Usage: "Skip nix flake update, just run nh os switch",
					},
				},
				Action: func(c *cli.Context) error {
					return handleDeploy(c.Bool("no-update"))
				},
			},
			{
				Name:  "rollback",
				Usage: "Roll back to the previous NixOS generation",
				Action: func(c *cli.Context) error {
					return handleRollback()
				},
			},
		},
		// Default action with no subcommand = switch
		Action: func(c *cli.Context) error {
			return handleDeploy(false)
		},
	}
}

func handleDeploy(noUpdate bool) error {
	flakeDir := os.Getenv("NH_FLAKE")
	if flakeDir == "" {
		flakeDir = os.Getenv("NH_OS_FLAKE")
	}
	if flakeDir == "" {
		return fmt.Errorf("NH_FLAKE is not set — set it to your nixos flake directory")
	}

	if !noUpdate {
		fmt.Println("► Updating homelab-daemon flake input...")
		flakeUpdateCmd := exec.Command("nix", "flake", "update", "homelab-daemon", "--flake", flakeDir)
		flakeUpdateCmd.Stdout = os.Stdout
		flakeUpdateCmd.Stderr = os.Stderr
		if err := flakeUpdateCmd.Run(); err != nil {
			return fmt.Errorf("nix flake update failed: %w", err)
		}
	}

	fmt.Println("► Running nh os switch...")
	switchCmd := exec.Command("nh", "os", "switch")
	switchCmd.Stdout = os.Stdout
	switchCmd.Stderr = os.Stderr
	return switchCmd.Run()
}

func handleRollback() error {
	fmt.Println("► Running nh os rollback...")
	cmd := exec.Command("nh", "os", "rollback")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ── completion ───────────────────────────────────────────────────────────────

func completionCmd() *cli.Command {
	return &cli.Command{
		Name:  "completion",
		Usage: "Generate shell completion script",
		Subcommands: []*cli.Command{
			{
				Name:  "bash",
				Usage: "Generate bash completion script",
				Action: func(c *cli.Context) error {
					fmt.Println(`# Add to ~/.bashrc:
# eval "$(homelab completion bash)"
_homelab() {
    COMPREPLY=($(homelab "${COMP_WORDS[@]:1}" --generate-bash-completion 2>/dev/null))
    return 0
}
complete -F _homelab homelab`)
					return nil
				},
			},
			{
				Name:  "zsh",
				Usage: "Generate zsh completion script",
				Action: func(c *cli.Context) error {
					fmt.Println(`# Add to ~/.zshrc:
# autoload -U compinit && compinit  (required once in ~/.zshrc)
# eval "$(homelab completion zsh)"
_homelab() {
    local -a completions
    completions=(${(f)"$(homelab ${words[2,-1]} --generate-bash-completion 2>/dev/null)"})
    compadd -a completions
}
compdef _homelab homelab`)
					return nil
				},
			},
		},
	}
}

// ── merge-config ─────────────────────────────────────────────────────────────

func mergeConfigCmd() *cli.Command {
	return &cli.Command{
		Name:   "merge-config",
		Usage:  "Merge default service settings into services.yaml",
		Hidden: true,
		Action: func(c *cli.Context) error {
			cmd := exec.Command("homelab-daemon", append([]string{"merge-config"}, c.Args().Slice()...)...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		},
	}
}

// ── input helpers ────────────────────────────────────────────────────────────

func readLine(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	if err := exec.Command("stty", "-F", "/dev/tty", "-echo").Run(); err != nil {
		return "", fmt.Errorf("stty -echo: %w", err)
	}
	defer func() { _ = exec.Command("stty", "-F", "/dev/tty", "echo").Run() }()
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	fmt.Fprintln(os.Stderr)
	return strings.TrimSpace(line), nil
}
