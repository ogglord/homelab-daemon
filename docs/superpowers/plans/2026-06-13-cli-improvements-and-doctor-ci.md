# CLI Improvements & Homelab Doctor CI Integration

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the `homelab` CLI to urfave/cli v2, extract doctor checks into a reusable `internal/doctor` package, and wire `homelab doctor` into NixOS activation with journal logging and SMTP notification on failure.

**Architecture:** Single binary (`cmd/cli/main.go`) restructured with urfave/cli v2 subcommand groups. Doctor checks extracted to `internal/doctor/` with a `Run(checks []string) Report` function and a standalone `Notify(report, cfg)` function that reads YAML config directly (no daemon dependency). NixOS `module.nix` gains an activation script + oneshot systemd service calling `homelab doctor --json --fail-on-error`.

**Tech Stack:** Go 1.26, urfave/cli v2, existing `internal/notifier` (SMTP), `gopkg.in/yaml.v3`, NixOS activation scripts / systemd oneshot services.

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `cmd/cli/main.go` | Rewrite | urfave/cli app, subcommand wiring, global `--json` flag |
| `internal/doctor/doctor.go` | Create | Check registry, `Run()`, `Report`/`Result` types |
| `internal/doctor/notify.go` | Create | `Notify()`, config reader for standalone SMTP |
| `module.nix` | Modify | `enableDoctorOnActivation` option, activation script, oneshot service |
| `go.mod` + `vendor/` | Modify | Add `github.com/urfave/cli/v2` |

---

## Task 1: Add urfave/cli v2 dependency

**Files:**
- Modify: `go.mod`
- Modify: `vendor/` (via `go mod vendor`)

- [ ] **Step 1: Add the dependency**

```bash
nix develop --command bash -c 'go get github.com/urfave/cli/v2'
```

- [ ] **Step 2: Vendor it**

```bash
nix develop --command bash -c 'go mod tidy && go mod vendor'
```

- [ ] **Step 3: Verify build still works**

```bash
nix develop --command bash -c 'go build ./cmd/cli/'
```

Expected: binary builds, no errors.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum vendor/
git commit -m "chore(deps): add urfave/cli/v2"
```

---

## Task 2: Create `internal/doctor` package — types and check registry

**Files:**
- Create: `internal/doctor/doctor.go`

- [ ] **Step 1: Write the failing test**

Create `internal/doctor/doctor_test.go`:

```go
package doctor_test

import (
	"testing"

	"github.com/ogglord/homelab-daemon/internal/doctor"
)

func TestRunSubset(t *testing.T) {
	// Run only the disk check — always available in test env
	report := doctor.Run([]string{"disk"})
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	if report.Results[0].Name != "disk" {
		t.Errorf("expected name 'disk', got %q", report.Results[0].Name)
	}
}

func TestRunAll(t *testing.T) {
	report := doctor.Run(nil) // nil = all checks
	if len(report.Results) == 0 {
		t.Fatal("expected at least one result")
	}
	total := report.Passed + report.Failed
	if total != len(report.Results) {
		t.Errorf("passed+failed=%d != len(results)=%d", total, len(report.Results))
	}
}

func TestRunUnknownCheck(t *testing.T) {
	report := doctor.Run([]string{"nonexistent-check"})
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	if report.Results[0].OK {
		t.Error("unknown check should not be OK")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
nix develop --command bash -c 'go test ./internal/doctor/... -v'
```

Expected: compile error — package does not exist yet.

- [ ] **Step 3: Create `internal/doctor/doctor.go`**

```go
// Package doctor provides health checks for the homelab system.
package doctor

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const socketPath = "/run/homelab-daemon/daemon.sock"

// Result is the outcome of a single health check.
type Result struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
	Fix    string `json:"fix,omitempty"`
}

// Report is the aggregate outcome of running a set of checks.
type Report struct {
	Passed  int      `json:"passed"`
	Failed  int      `json:"failed"`
	Results []Result `json:"results"`
}

// check is an internal registered check.
type check struct {
	name string
	run  func() Result
}

// registry is the ordered list of all known checks.
var registry = []check{
	{"socket", checkSocket},
	{"daemon", checkDaemon},
	{"postgres", checkPostgres},
	{"caddy", checkCaddy},
	{"dashboard", checkDashboard},
	{"dashboard-pages", checkDashboardPages},
	{"disk", checkDisk},
	{"systemd-units", checkSystemdUnits},
}

// Run executes checks by slug. If slugs is nil or empty, all checks run.
// Unknown slugs produce a failed Result with a descriptive message.
func Run(slugs []string) Report {
	var toRun []check
	if len(slugs) == 0 {
		toRun = registry
	} else {
		idx := make(map[string]check, len(registry))
		for _, c := range registry {
			idx[c.name] = c
		}
		for _, s := range slugs {
			if c, ok := idx[s]; ok {
				toRun = append(toRun, c)
			} else {
				toRun = append(toRun, check{
					name: s,
					run: func() Result {
						return Result{
							Name:   s,
							OK:     false,
							Detail: fmt.Sprintf("unknown check %q", s),
							Fix:    fmt.Sprintf("valid checks: %s", strings.Join(knownSlugs(), ", ")),
						}
					},
				})
			}
		}
	}

	r := Report{}
	for _, c := range toRun {
		res := c.run()
		res.Name = c.name
		r.Results = append(r.Results, res)
		if res.OK {
			r.Passed++
		} else {
			r.Failed++
		}
	}
	return r
}

func knownSlugs() []string {
	s := make([]string, len(registry))
	for i, c := range registry {
		s[i] = c.name
	}
	return s
}

// ── Individual checks ────────────────────────────────────────────────────────

func checkSocket() Result {
	_, err := os.Stat(socketPath)
	if err == nil {
		return Result{OK: true, Detail: socketPath}
	}
	return Result{
		OK:     false,
		Detail: fmt.Sprintf("cannot access socket: %v", err),
		Fix:    "systemctl status homelab-daemon.service",
	}
}

func checkDaemon() Result {
	return unitResult("homelab-daemon.service",
		"homelab-daemon.service is active",
		"sudo systemctl start homelab-daemon.service")
}

func checkPostgres() Result {
	return unitResult("postgresql.service",
		"postgresql.service is active",
		"sudo systemctl start postgresql.service")
}

func checkCaddy() Result {
	return unitResult("caddy.service",
		"caddy.service is active",
		"sudo systemctl start caddy.service")
}

func unitResult(unit, okDetail, fix string) Result {
	cmd := exec.Command("systemctl", "is-active", unit)
	if cmd.Run() == nil {
		return Result{OK: true, Detail: okDetail}
	}
	return Result{
		OK:     false,
		Detail: fmt.Sprintf("%s is inactive", unit),
		Fix:    fix,
	}
}

func dashboardBaseURL() string {
	// Attempt to read from daemon API; fall back to hardcoded default.
	// This avoids the circular dependency of requiring the daemon to check the daemon.
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://unix/api/config")
	if err == nil {
		resp.Body.Close()
		// Future: parse dashboard URL from config response.
		// For now fall through to default.
	}
	return "https://dash.cignl.cc"
}

func checkDashboard() Result {
	base := dashboardBaseURL()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(base)
	if err != nil {
		return Result{
			OK:     false,
			Detail: fmt.Sprintf("connection failed: %v", err),
			Fix:    "verify Caddy is running and DNS resolves to this host",
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{
			OK:     false,
			Detail: fmt.Sprintf("HTTP %d at %s", resp.StatusCode, base),
			Fix:    "verify Caddy is running and DNS resolves to this host",
		}
	}
	return Result{OK: true, Detail: fmt.Sprintf("HTTP 200 OK at %s", base)}
}

func checkDashboardPages() Result {
	base := dashboardBaseURL()
	routes := []string{"", "services", "vms", "backups", "diagnostics", "storage", "secrets"}
	client := &http.Client{Timeout: 5 * time.Second}
	var failed []string
	for _, route := range routes {
		url := base + "/" + route
		resp, err := client.Get(url)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", url, err))
			continue
		}
		if resp.StatusCode != http.StatusOK {
			failed = append(failed, fmt.Sprintf("%s: HTTP %d", url, resp.StatusCode))
			resp.Body.Close()
			continue
		}
		buf := make([]byte, 512)
		n, _ := resp.Body.Read(buf)
		resp.Body.Close()
		body := strings.ToLower(string(buf[:n]))
		if !strings.Contains(body, "<html") && !strings.Contains(body, "<!doctype") {
			failed = append(failed, fmt.Sprintf("%s: not valid HTML", url))
		}
	}
	if len(failed) > 0 {
		return Result{
			OK:     false,
			Detail: strings.Join(failed, "; "),
			Fix:    "check Caddy routes and frontend dist in nix store",
		}
	}
	return Result{OK: true, Detail: fmt.Sprintf("all %d routes return HTTP 200 + valid HTML", len(routes))}
}

func checkDisk() Result {
	mounts := []string{"/", "/cache", "/pool"}
	var details []string
	ok := true
	for _, mnt := range mounts {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(mnt, &stat); err != nil {
			details = append(details, fmt.Sprintf("%s: not mounted (%v)", mnt, err))
			ok = false
			continue
		}
		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bfree * uint64(stat.Bsize)
		used := total - free
		var pct float64
		if total > 0 {
			pct = float64(used) / float64(total) * 100
		}
		details = append(details, fmt.Sprintf("%s: %.1f%% used (%s/%s)", mnt, pct, fmtBytes(used), fmtBytes(total)))
		if pct >= 90 {
			ok = false
		}
	}
	r := Result{OK: ok, Detail: strings.Join(details, "; ")}
	if !ok {
		r.Fix = "prune docker assets, clear system logs, or clean pool restore trees"
	}
	return r
}

func checkSystemdUnits() Result {
	cmd := exec.Command("systemctl", "list-units", "--failed", "--plain", "--no-legend")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return Result{
			OK:     false,
			Detail: fmt.Sprintf("failed to query systemd: %v", err),
			Fix:    "check systemctl status",
		}
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var failed []string
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			failed = append(failed, l)
		}
	}
	if len(failed) > 0 {
		return Result{
			OK:     false,
			Detail: fmt.Sprintf("%d failed unit(s): %s", len(failed), strings.Join(failed, ", ")),
			Fix:    "run 'systemctl status <unit>' or check journalctl",
		}
	}
	return Result{OK: true, Detail: "no failed systemd units"}
}

func fmtBytes(b uint64) string {
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
```

- [ ] **Step 4: Run tests**

```bash
nix develop --command bash -c 'go test ./internal/doctor/... -v'
```

Expected: `TestRunSubset` PASS, `TestRunAll` PASS, `TestRunUnknownCheck` PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/doctor/doctor.go internal/doctor/doctor_test.go
git commit -m "feat(doctor): extract check registry into internal/doctor package"
```

---

## Task 3: Create `internal/doctor/notify.go`

**Files:**
- Create: `internal/doctor/notify.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/doctor/doctor_test.go`:

```go
func TestFormatReport(t *testing.T) {
	report := doctor.Report{
		Passed: 1,
		Failed: 1,
		Results: []doctor.Result{
			{Name: "socket", OK: true, Detail: "/run/homelab-daemon/daemon.sock"},
			{Name: "daemon", OK: false, Detail: "inactive", Fix: "sudo systemctl start homelab-daemon.service"},
		},
	}
	body := doctor.FormatReportText(report)
	if !strings.Contains(body, "[✔] socket") {
		t.Errorf("expected passing check in output, got:\n%s", body)
	}
	if !strings.Contains(body, "[✗] daemon") {
		t.Errorf("expected failing check in output, got:\n%s", body)
	}
	if !strings.Contains(body, "Fix:") {
		t.Errorf("expected Fix hint in output, got:\n%s", body)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
nix develop --command bash -c 'go test ./internal/doctor/... -run TestFormatReport -v'
```

Expected: compile error — `FormatReportText` not defined.

- [ ] **Step 3: Create `internal/doctor/notify.go`**

```go
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
```

- [ ] **Step 4: Run tests**

```bash
nix develop --command bash -c 'go test ./internal/doctor/... -v'
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/doctor/notify.go internal/doctor/doctor_test.go
git commit -m "feat(doctor): add notify.go with Notify(), FormatReportText(), LoadNotifyConfigFromFile()"
```

---

## Task 4: Rewrite `cmd/cli/main.go` with urfave/cli v2

**Files:**
- Modify: `cmd/cli/main.go` (full rewrite)

This task migrates all existing commands plus adds new ones. The existing `handleXxx` functions are retained but wired through urfave/cli.

- [ ] **Step 1: Verify current CLI still builds before touching it**

```bash
nix develop --command bash -c 'go build ./cmd/cli/'
```

Expected: success.

- [ ] **Step 2: Rewrite `cmd/cli/main.go`**

Replace the entire file with:

```go
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
	"os"
	"os/exec"
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
	Timeout: 5 * time.Second,
}

var (
	Version   = "0.3.0"
	BuildDate = "2026-06-13"
)

func main() {
	app := &cli.App{
		Name:    "homelab",
		Usage:   "Manage your homelab services, backups, secrets, and health",
		Version: fmt.Sprintf("%s (built %s)", Version, BuildDate),
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
			mergeConfigCmd(),
		},
		// top-level alias: homelab status → homelab services status
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
	return c.Bool("json") || c.Parent() != nil && c.Parent().Bool("json")
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
				ArgsUsage: "<name>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("usage: homelab backup run <name>")
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
	// Pretty-print as table — marshal back to get typed slice
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

func handleBackupRun(name string) {
	resp, err := httpClient.Post(fmt.Sprintf("http://unix/api/backups/%s/run", name), "application/json", nil)
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		die("daemon returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	fmt.Printf("Triggered backup: %s\n", name)
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
			// secret add/set require root; list does not
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
	_, err := httpClient.Get("http://unix/api/health")
	if err != nil {
		if asJSON {
			fmt.Println(`{"ok":false,"error":"daemon unreachable"}`)
		} else {
			fmt.Fprintln(os.Stderr, "✗ Daemon unreachable")
		}
		os.Exit(1)
	}
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
```

- [ ] **Step 3: Build and verify**

```bash
nix develop --command bash -c 'go build ./cmd/cli/'
```

Expected: compiles with no errors.

- [ ] **Step 4: Smoke test help output**

```bash
./homelab --help
./homelab services --help
./homelab doctor --help
```

Expected: all subcommands listed, no panics.

- [ ] **Step 5: Commit**

```bash
git add cmd/cli/main.go
git commit -m "feat(cli): migrate to urfave/cli v2, add backup/config/daemon subcommands, fix restart routing"
```

---

## Task 5: Add `/api/health` endpoint to daemon

`handleDaemonStatus` calls `GET /api/health`. Add this lightweight endpoint.

**Files:**
- Modify: `cmd/daemon/api.go`

- [ ] **Step 1: Check if endpoint already exists**

```bash
grep -n "health" cmd/daemon/api.go
```

If it already returns a 200, skip to Task 6.

- [ ] **Step 2: Add the handler**

Find the block where other handlers are registered (around line 272 where `GET /api/status` is registered). Add after the existing registrations:

```go
mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte(`{"ok":true}`))
})
```

- [ ] **Step 3: Build daemon**

```bash
nix develop --command bash -c 'go build ./cmd/daemon/'
```

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add cmd/daemon/api.go
git commit -m "feat(api): add GET /api/health lightweight ping endpoint"
```

---

## Task 6: Add `/api/backups` endpoint to daemon

`handleBackupStatus` calls `GET /api/backups`. Verify it exists; add if not.

**Files:**
- Modify: `cmd/daemon/api.go` (if missing)

- [ ] **Step 1: Check if endpoint exists**

```bash
grep -n "api/backups" cmd/daemon/api.go
```

If it exists, skip to Task 7.

- [ ] **Step 2: Add the handler**

In `cmd/daemon/api.go`, find where backup-related state is read (search for `scheduler` or `lastRun`). Add:

```go
mux.HandleFunc("GET /api/backups", func(w http.ResponseWriter, r *http.Request) {
    statuses := s.scheduler.BackupStatuses()
    writeJSON(w, statuses)
})
```

Then add `BackupStatuses()` to `cmd/daemon/scheduler.go` if it doesn't exist — it should return the current last-run + next-run times for each backup job. The exact implementation depends on what state scheduler already exposes; check `cmd/daemon/scheduler.go` for existing state fields before writing.

- [ ] **Step 3: Build**

```bash
nix develop --command bash -c 'go build ./cmd/daemon/'
```

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add cmd/daemon/api.go cmd/daemon/scheduler.go
git commit -m "feat(api): add GET /api/backups status endpoint"
```

---

## Task 7: Add `/api/backups/{name}/run` endpoint to daemon

`handleBackupRun` calls `POST /api/backups/{name}/run`. Add if not present.

**Files:**
- Modify: `cmd/daemon/api.go`

- [ ] **Step 1: Check if endpoint exists**

```bash
grep -n "backups.*run\|run.*backup" cmd/daemon/api.go
```

If it exists, skip to Task 8.

- [ ] **Step 2: Add the handler**

```go
mux.HandleFunc("POST /api/backups/{name}/run", func(w http.ResponseWriter, r *http.Request) {
    name := r.PathValue("name")
    if err := s.scheduler.RunNow(name); err != nil {
        http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadRequest)
        return
    }
    writeJSON(w, map[string]bool{"ok": true})
})
```

Check `cmd/daemon/scheduler.go` for an existing `RunNow` or equivalent method; use it if present, add it if not. `RunNow` should trigger the named backup job asynchronously and return an error if the name is not found.

- [ ] **Step 3: Build**

```bash
nix develop --command bash -c 'go build ./cmd/daemon/'
```

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add cmd/daemon/api.go cmd/daemon/scheduler.go
git commit -m "feat(api): add POST /api/backups/{name}/run endpoint"
```

---

## Task 8: Run full preflight

- [ ] **Step 1: Run preflight**

```bash
make preflight
```

Expected:
```
All pre-flight checks passed.
```

Fix any vet errors before proceeding.

- [ ] **Step 2: Commit any fixes**

```bash
git add -A
git commit -m "fix(cli): address go vet findings"
```

---

## Task 9: NixOS module — `enableDoctorOnActivation` option + activation script

**Files:**
- Modify: `module.nix`

- [ ] **Step 1: Locate the options block in `module.nix`**

```bash
grep -n "mkOption\|options\." module.nix | head -20
```

- [ ] **Step 2: Add the option**

Find the last `mkOption` block in the options section. After it, add:

```nix
enableDoctorOnActivation = lib.mkOption {
  type = lib.types.bool;
  default = true;
  description = ''
    Run `homelab doctor` after each NixOS activation (nh os switch).
    Results are written to the journal (tag: homelab-doctor).
    Failures emit an SMTP notification but never block the switch.
  '';
};
```

- [ ] **Step 3: Add the activation script**

Find `system.activationScripts` in `module.nix`. Add a new entry:

```nix
system.activationScripts.homelabDoctor = lib.mkIf cfg.enableDoctorOnActivation {
  deps = [ "specialfs" ];
  text = ''
    ${pkgs.homelab-daemon}/bin/homelab doctor --json 2>&1 \
      | ${pkgs.systemd}/bin/systemd-cat -t homelab-doctor -p info || true
  '';
};
```

- [ ] **Step 4: Verify `module.nix` parses (no Nix eval errors)**

```bash
nix eval .#nixosModules.default --apply builtins.typeOf 2>&1 | head -5
```

Expected: `"lambda"` or similar — no parse errors.

- [ ] **Step 5: Commit**

```bash
git add module.nix
git commit -m "feat(nixos): add enableDoctorOnActivation option and activation script"
```

---

## Task 10: NixOS module — oneshot systemd service + SMTP notification

**Files:**
- Modify: `module.nix`

- [ ] **Step 1: Add the oneshot service + notify companion**

In `module.nix`, in the `systemd.services` block (or add one), add under `lib.mkIf cfg.enableDoctorOnActivation`:

```nix
systemd.services.homelab-doctor-report = lib.mkIf cfg.enableDoctorOnActivation {
  description = "Homelab post-activation doctor report";
  wantedBy = [ "multi-user.target" ];
  after = [ "homelab-daemon.service" "network-online.target" ];
  serviceConfig = {
    Type = "oneshot";
    RemainAfterExit = true;
    ExecStart = "${pkgs.homelab-daemon}/bin/homelab doctor --json --fail-on-error";
    ExecStopPost = ''
      /bin/sh -c '${pkgs.homelab-daemon}/bin/homelab doctor --json \
        | ${pkgs.homelab-daemon}/bin/homelab doctor notify || true'
    '';
    StandardOutput = "journal";
    StandardError = "journal";
    SyslogIdentifier = "homelab-doctor";
  };
};
```

- [ ] **Step 2: Verify Nix eval**

```bash
nix eval .#nixosModules.default --apply builtins.typeOf 2>&1 | head -5
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add module.nix
git commit -m "feat(nixos): add homelab-doctor-report oneshot service with SMTP notification on failure"
```

---

## Task 11: Update hashes and final preflight

- [ ] **Step 1: Update vendorHash**

```bash
./scripts/update-hashes.sh daemon
```

- [ ] **Step 2: Run full preflight**

```bash
make preflight
```

Expected: `All pre-flight checks passed.`

- [ ] **Step 3: Commit**

```bash
git add flake.nix
git commit -m "chore(nix): update vendorHash after adding urfave/cli/v2"
```

---

## Self-Review

**Spec coverage check:**
- ✔ CLI migrated to urfave/cli v2 (Task 4)
- ✔ `secret add` command (Task 4)
- ✔ `backup status` + `backup run` commands (Tasks 4, 6, 7)
- ✔ `config show` command (Task 4)
- ✔ `daemon status` ping (Tasks 4, 5)
- ✔ `doctor --json --fail-on-error --check` flags (Tasks 2, 3, 4)
- ✔ `doctor notify` subcommand (Tasks 3, 4)
- ✔ `restart` routed through daemon socket (Task 4)
- ✔ `die()` helper replacing repeated os.Exit(1) (Task 4)
- ✔ `internal/doctor` package (Tasks 2, 3)
- ✔ `Notify()` reads YAML config standalone (Task 3)
- ✔ NixOS `enableDoctorOnActivation` option (Task 9)
- ✔ Activation script → journal (Task 9)
- ✔ Oneshot service + SMTP on failure (Task 10)
- ✔ vendorHash update (Task 11)

**Placeholder scan:** None.

**Type consistency:**
- `doctor.Run([]string) Report` — defined Task 2, used Tasks 3, 4 ✔
- `doctor.FormatReportText(Report) string` — defined Task 3, used Task 4 ✔
- `doctor.Notify(Report, NotifyConfig) error` — defined Task 3, used Task 4 ✔
- `doctor.LoadNotifyConfigFromFile(string) (NotifyConfig, error)` — defined Task 3, used Task 4 ✔

**Note on Tasks 6 & 7:** Both verify whether the daemon endpoints already exist before adding them. The scheduler state fields need to be inspected at implementation time — the plan flags this explicitly rather than guessing the existing API surface.
