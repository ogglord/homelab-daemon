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
const systemctlPath = "/run/current-system/sw/bin/systemctl"

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
			slug := s // capture loop variable
			if c, ok := idx[slug]; ok {
				toRun = append(toRun, c)
			} else {
				toRun = append(toRun, check{
					name: slug,
					run: func() Result {
						return Result{
							Name:   slug,
							OK:     false,
							Detail: fmt.Sprintf("unknown check %q", slug),
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
	cmd := exec.Command(systemctlPath, "is-active", unit)
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
	// TODO: read from daemon config once /api/config exposes the dashboard URL.
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
	cmd := exec.Command(systemctlPath, "list-units", "--failed", "--plain", "--no-legend")
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
