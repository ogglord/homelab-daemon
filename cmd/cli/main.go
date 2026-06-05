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
	"syscall"
	"text/tabwriter"
	"time"
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
	Version   = "0.2.0"
	BuildDate = "2026-05-27"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[1:]

	if command == "--version" || command == "-v" || command == "version" {
		fmt.Printf("homelab CLI version %s (built %s)\n", Version, BuildDate)
		os.Exit(0)
	}

	if command == "merge-config" {
		handleMergeConfig(os.Args[2:])
		os.Exit(0)
	}

	if command == "doctor" {
		handleDoctor()
		os.Exit(0)
	}

	if command == "secret" {
		checkRoot()
		if len(args) < 2 {
			printSecretUsage()
			os.Exit(1)
		}
		handleSecret(args[1], args[2:])
		os.Exit(0)
	}

	// Support "homelab services <cmd>"
	if command == "services" {
		if len(args) < 2 {
			printUsage()
			os.Exit(1)
		}
		command = args[1]
		args = args[1:]
	}

	switch command {
	case "status":
		handleStatus()
	case "start":
		if len(args) < 2 {
			fmt.Println("Usage: homelab services start <unit>")
			os.Exit(1)
		}
		handleStart(args[1])
	case "stop":
		if len(args) < 2 {
			fmt.Println("Usage: homelab services stop <unit>")
			os.Exit(1)
		}
		handleStop(args[1])
	case "restart":
		if len(args) < 2 {
			fmt.Println("Usage: homelab services restart <unit>")
			os.Exit(1)
		}
		handleRestart(args[1])
	case "enable":
		if len(args) < 2 {
			fmt.Println("Usage: homelab services enable <unit>")
			os.Exit(1)
		}
		handleEnable(args[1], true)
	case "disable":
		if len(args) < 2 {
			fmt.Println("Usage: homelab services disable <unit>")
			os.Exit(1)
		}
		handleEnable(args[1], false)
	case "logs":
		if len(args) < 2 {
			fmt.Println("Usage: homelab services logs <unit>")
			os.Exit(1)
		}
		handleLogs(args[1])

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf("homelab CLI version %s (built %s)\n\n", Version, BuildDate)
	fmt.Println(`Usage: homelab [services] <command> [unit]

Commands:
  status               List status of all managed services
  start <unit>         Start a service
  stop <unit>          Stop a service
  restart <unit>       Restart a service
  enable <unit>        Enable a service (autostart)
  disable <unit>       Disable a service
  logs <unit>          Tail logs for a service

  secret list          List all declared secrets and their status
  secret set [name]    Set/rotate a secret value (interactive picker if no name)

  doctor               Run diagnostic health and smoke checks
  version              Print version information
  merge-config         Merge default service settings into services.yaml`)
}

func handleMergeConfig(args []string) {
	cmd := exec.Command("homelab-daemon", append([]string{"merge-config"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

type serviceStatus struct {
	Unit          string   `json:"unit"`
	Enabled       bool     `json:"enabled"`
	Active        bool     `json:"active"`
	UserStopped   bool     `json:"user_stopped"`
	Restart       string   `json:"restart"`
	Order         int      `json:"order"`
	FailureCount  int      `json:"failure_count"`
	BackoffUntil  string   `json:"backoff_until"`
	BlockedReason string   `json:"blocked_reason"`
}

func handleStatus() {
	resp, err := httpClient.Get("http://unix/api/status")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error contacting daemon: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Daemon returned status %d\n", resp.StatusCode)
		os.Exit(1)
	}

	var services []serviceStatus
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding response: %v\n", err)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "Error contacting daemon: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Daemon returned status %d\n", resp.StatusCode)
		os.Exit(1)
	}
	fmt.Printf("Successfully started %s\n", unit)
}

func handleStop(unit string) {
	resp, err := httpClient.Post(fmt.Sprintf("http://unix/api/stop/%s", unit), "application/json", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error contacting daemon: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Daemon returned status %d\n", resp.StatusCode)
		os.Exit(1)
	}
	fmt.Printf("Successfully stopped %s\n", unit)
}

func handleRestart(unit string) {
	// homelab-dash uses systemctl restart, which is sufficient because
	// daemon manages user_stopped state; restarting an active process doesn't change intent.
	cmd := exec.Command("systemctl", "restart", unit)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error restarting service: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Successfully restarted %s\n", unit)
}

func handleEnable(unit string, enable bool) {
	payload := map[string]bool{"enabled": enable}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("http://unix/api/config/%s", unit), bytes.NewReader(b))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error contacting daemon: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Daemon returned status %d\n", resp.StatusCode)
		os.Exit(1)
	}

	action := "disabled"
	if enable {
		action = "enabled"
	}
	fmt.Printf("Successfully %s %s\n", action, unit)
}

func handleLogs(unit string) {
	cmd := exec.Command("journalctl", "-fu", unit)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running journalctl: %v\n", err)
		os.Exit(1)
	}
}

func handleDoctor() {
	fmt.Println("🏥 Starting Homelab Diagnostics (homelab doctor)...")
	fmt.Println("====================================================")
	time.Sleep(100 * time.Millisecond)

	allPassed := true

	printResult := func(name string, success bool, details string, recommendation string) {
		if success {
			fmt.Printf(" [✔] %s\n", name)
			if details != "" {
				fmt.Printf("     %s\n", details)
			}
		} else {
			fmt.Printf(" [✗] %s\n", name)
			if details != "" {
				fmt.Printf("     Reason: %s\n", details)
			}
			if recommendation != "" {
				fmt.Printf("     Recommendation: %s\n", recommendation)
			}
			allPassed = false
		}
	}

	// 1. Daemon Socket reachability
	_, err := os.Stat(socketPath)
	if err == nil {
		printResult("Daemon socket reachable", true, socketPath, "")
	} else {
		printResult("Daemon socket reachable", false, fmt.Sprintf("Cannot access socket: %v", err), "Verify homelab-daemon is running with 'systemctl status homelab-daemon.service'.")
	}

	// 2. Daemon Service Status
	if isUnitActive("homelab-daemon.service") {
		printResult("Daemon service is running", true, "homelab-daemon.service is active", "")
	} else {
		printResult("Daemon service is running", false, "homelab-daemon.service is inactive", "Start it using 'sudo systemctl start homelab-daemon.service'.")
	}

	// 3. Postgres Service Status
	if isUnitActive("postgresql.service") {
		printResult("Postgres database is running", true, "postgresql.service is active", "")
	} else {
		printResult("Postgres database is running", false, "postgresql.service is inactive", "Start it using 'sudo systemctl start postgresql.service'.")
	}

	// 4. Caddy Service Status
	if isUnitActive("caddy.service") {
		printResult("Caddy reverse proxy is running", true, "caddy.service is active", "")
	} else {
		printResult("Caddy reverse proxy is running", false, "caddy.service is inactive", "Start it using 'sudo systemctl start caddy.service'.")
	}

	// 5. Dashboard HTTPS Responsiveness
	msg, ok := checkDashboard()
	printResult("Dashboard web response check", ok, msg, "Verify Caddy is running and 'dash.cignl.cc' DNS resolves to this host.")

	// 6. Dashboard SPA page checks — every route returns 200 + valid HTML
	pagesMsg, pagesOk := checkDashPages()
	printResult("Dashboard page checks (all routes return 200 + valid HTML)", pagesOk, pagesMsg, "Check Caddy routes and frontend dist in nix store.")

	// 7. Disk Space checks
	diskMounts := []string{"/", "/cache", "/pool"}
	var diskDetails []string
	diskOk := true
	for _, mnt := range diskMounts {
		str, ok := checkDiskSpace(mnt)
		if !ok {
			diskOk = false
		}
		diskDetails = append(diskDetails, str)
	}
	printResult("Filesystem capacity checks", diskOk, strings.Join(diskDetails, "\n     "), "Prune unnecessary docker assets, clear system logs, or clean pool restore trees.")

	// 8. Systemd Failed Units Check
	failedMsg, failedOk := checkFailedUnits()
	printResult("Failed systemd units check", failedOk, failedMsg, "Run 'systemctl status <unit>' or check journalctl logs to diagnose failed units.")

	fmt.Println("====================================================")
	if allPassed {
		fmt.Println("🎉 Everything is healthy! Your homelab is in perfect shape.")
	} else {
		fmt.Println("⚠️ Some checks failed. Review recommendations above to troubleshoot.")
	}
}

func isUnitActive(unit string) bool {
	cmd := exec.Command("systemctl", "is-active", unit)
	err := cmd.Run()
	return err == nil
}

func checkDashboard() (string, bool) {
	return doCheckURL("https://dash.cignl.cc", "")
}

func checkDashPages() (string, bool) {
	pages := []string{
		"",
		"services",
		"vms",
		"backups",
		"diagnostics",
		"storage",
		"secrets",
	}
	var failed []string
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	for _, path := range pages {
		url := fmt.Sprintf("https://dash.cignl.cc/%s", path)
		resp, err := client.Get(url)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: connection failed: %v", url, err))
			continue
		}
		if resp.StatusCode != http.StatusOK {
			failed = append(failed, fmt.Sprintf("%s: HTTP %d", url, resp.StatusCode))
			resp.Body.Close()
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !isHTML(body) {
			failed = append(failed, fmt.Sprintf("%s: response is not valid HTML (no <html> or <!DOCTYPE>)", url))
		}
	}
	if len(failed) > 0 {
		return strings.Join(failed, "\n     "), false
	}
	return fmt.Sprintf("All %d pages return HTTP 200 with valid HTML", len(pages)), true
}

func doCheckURL(baseURL, path string) (string, bool) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	url := baseURL
	if path != "" {
		url = baseURL + "/" + path
	}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Sprintf("Connection failed: %v", err), false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("HTTP %d at %s", resp.StatusCode, url), false
	}
	return fmt.Sprintf("HTTP 200 OK at %s", url), true
}

func isHTML(body []byte) bool {
	s := string(body)
	return strings.Contains(s, "<html") || strings.Contains(s, "<!DOCTYPE") || strings.Contains(s, "<!doctype")
}

func checkDiskSpace(path string) (string, bool) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return fmt.Sprintf("%s: not mounted or inaccessible (%v)", path, err), false
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := total - free
	var pct float64
	if total > 0 {
		pct = float64(used) / float64(total) * 100.0
	}
	return fmt.Sprintf("%s: %.1f%% used (%s/%s)", path, pct, fmtBytes(used), fmtBytes(total)), pct < 90.0
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

func printSecretUsage() {
	fmt.Println(`Usage: homelab secret <command> [name]

Commands:
  list          List all declared secrets and their status
  set [name]    Set/rotate a secret value (interactive picker if no name)

Requires sudo privileges.`)
}

// checkRoot exits if not running as root (sudo).
func checkRoot() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "Error: homelab secret requires sudo privileges.")
		os.Exit(1)
	}
}

// readPassword reads a secret value without echoing characters.
func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	// Disable terminal echo via stty.
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

type secretEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Present     bool   `json:"present"`
	ModifiedAt  string `json:"modified_at"`
	Preview     string `json:"preview"`
}

func handleSecret(subcommand string, args []string) {
	switch subcommand {
	case "list":
		handleSecretList()
	case "set":
		handleSecretSet(args)
	default:
		fmt.Printf("Unknown secret command: %s\n", subcommand)
		printSecretUsage()
		os.Exit(1)
	}
}

func handleSecretList() {
	resp, err := httpClient.Get("http://unix/api/secrets")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error contacting daemon: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Daemon returned status %d: %s\n", resp.StatusCode, strings.TrimSpace(string(body)))
		os.Exit(1)
	}

	var result struct {
		Secrets       []secretEntry `json:"secrets"`
		DeployPending bool          `json:"deploy_pending"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding response: %v\n", err)
		os.Exit(1)
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

func handleSecretSet(args []string) {
	if len(args) >= 1 && args[0] != "" {
		// Direct set: homelab secret set <name>
		setSecretByName(args[0])
		return
	}

	// Interactive picker: homelab secret set (no name)
	name := pickSecretInteractive()
	if name == "" {
		return
	}
	setSecretByName(name)
}

func pickSecretInteractive() string {
	resp, err := httpClient.Get("http://unix/api/secrets")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error contacting daemon: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		Secrets []secretEntry `json:"secrets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding response: %v\n", err)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}
	if val == "" {
		fmt.Fprintln(os.Stderr, "Error: value must not be empty.")
		os.Exit(1)
	}

	body := map[string]string{"value": val}
	b, err := json.Marshal(body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding request: %v\n", err)
		os.Exit(1)
	}

	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://unix/api/secrets/%s", name), bytes.NewReader(b))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error contacting daemon: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Daemon returned status %d: %s\n", resp.StatusCode, strings.TrimSpace(string(respBody)))
		os.Exit(1)
	}

	var result struct {
		Success       bool `json:"success"`
		DeployPending bool `json:"deploy_pending"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding response: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Secret '%s' updated.", name)
	if result.DeployPending {
		fmt.Println(" Deploy pending — run 'nh os switch' to apply.")
	} else {
		fmt.Println()
	}
}

func checkFailedUnits() (string, bool) {
	cmd := exec.Command("systemctl", "list-units", "--failed", "--plain", "--no-legend")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return fmt.Sprintf("Failed to check failed units: %v", err), false
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var failed []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			failed = append(failed, line)
		}
	}
	if len(failed) > 0 {
		return fmt.Sprintf("%d failed systemd unit(s) detected:\n     %s", len(failed), strings.Join(failed, "\n     ")), false
	}
	return "No failed systemd units detected", true
}
