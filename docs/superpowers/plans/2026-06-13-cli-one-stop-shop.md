# CLI One-Stop-Shop Expansion

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand the `homelab` CLI into a complete operational tool by adding deploy/rollback, shell completions, VM management, storage status, update checking, and notification testing.

**Architecture:** All new commands are added to `cmd/cli/main.go` using the existing urfave/cli v2 pattern. Each new command group follows the same `xxxCmd() *cli.Command` + `handleXxx()` split already established. No new packages needed — all commands call existing daemon API endpoints or shell out to `nh`/`nix`.

**Tech Stack:** Go 1.26, urfave/cli v2, existing daemon API (GET /api/vms, POST /api/vms/{name}/{action}, GET /api/storage, GET /api/updates, POST /api/updates/check), `nh` CLI, `nix` CLI.

---

## File Map

| File | Action | What changes |
|---|---|---|
| `cmd/cli/main.go` | Modify | Add deployCmd, vmCmd, storageCmd, updateCmd, notifyCmd; enable shell completions |

---

## Task 1: Shell completions

**Files:**
- Modify: `cmd/cli/main.go`

urfave/cli v2 generates bash/zsh/fish completions automatically. Only requires setting `EnableBashCompletion: true` on the app and adding a `completion` command.

- [ ] **Step 1: Verify current app struct in main.go**

```bash
grep -n "cli.App{" /home/ogge/homelab-daemon/cmd/cli/main.go
```

- [ ] **Step 2: Enable bash completion on the App**

In `main()`, find the `app := &cli.App{` block. Add `EnableBashCompletion: true` to the struct:

```go
app := &cli.App{
    Name:                 "homelab",
    Usage:                "Manage your homelab services, backups, secrets, and health",
    Version:              fmt.Sprintf("%s (built %s)", Version, BuildDate),
    EnableBashCompletion: true,
    Flags: []cli.Flag{
```

- [ ] **Step 3: Add completion subcommand to Commands list**

After `mergeConfigCmd()` in the Commands slice, add:

```go
completionCmd(),
```

- [ ] **Step 4: Add completionCmd function**

Add before the `mergeConfigCmd` function:

```go
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
_homelab_completion() {
    local cur prev words cword
    _init_completion || return
    COMPREPLY=($(COMP_LINE="${COMP_LINE}" COMP_POINT="${COMP_POINT}" homelab --generate-bash-completion))
    return 0
}
complete -F _homelab_completion homelab`)
					return nil
				},
			},
			{
				Name:  "zsh",
				Usage: "Generate zsh completion script",
				Action: func(c *cli.Context) error {
					fmt.Println(`# Add to ~/.zshrc:
# eval "$(homelab completion zsh)"
_homelab() {
    local -a completions
    completions=($(COMP_LINE="${COMP_LINE}" COMP_POINT="${COMP_POINT}" homelab --generate-bash-completion))
    compadd -a completions
}
compdef _homelab homelab`)
					return nil
				},
			},
		},
	}
}
```

- [ ] **Step 5: Build and verify**

```bash
cd /home/ogge/homelab-daemon && nix develop --command bash -c 'go build ./cmd/cli/'
./homelab completion bash
./homelab --generate-bash-completion
```

Expected: completion script printed, no panics; `--generate-bash-completion` outputs subcommand names.

- [ ] **Step 6: Commit**

```bash
git add cmd/cli/main.go && git commit -m "feat(cli): add shell completion command (bash/zsh)"
```

---

## Task 2: `homelab deploy` and `homelab rollback`

**Files:**
- Modify: `cmd/cli/main.go`

Calls `nix flake update homelab-daemon` in `$NH_FLAKE` dir, then `nh os switch`. Rollback calls `nh os rollback`. Both stream output live.

- [ ] **Step 1: Add deployCmd and rollbackCmd to Commands slice in main()**

After `doctorCmd()`, before `mergeConfigCmd()`:

```go
deployCmd(),
```

- [ ] **Step 2: Add deployCmd function**

```go
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
		updateCmd := exec.Command("nix", "flake", "update", "homelab-daemon", "--flake", flakeDir)
		updateCmd.Stdout = os.Stdout
		updateCmd.Stderr = os.Stderr
		if err := updateCmd.Run(); err != nil {
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
```

- [ ] **Step 3: Build and smoke test**

```bash
cd /home/ogge/homelab-daemon && nix develop --command bash -c 'go build ./cmd/cli/'
./homelab deploy --help
./homelab deploy switch --help
```

Expected: help text shown, `--no-update` flag listed.

- [ ] **Step 4: Commit**

```bash
git add cmd/cli/main.go && git commit -m "feat(cli): add deploy switch/rollback commands wrapping nh os"
```

---

## Task 3: `homelab vm` commands

**Files:**
- Modify: `cmd/cli/main.go`

Wraps `GET /api/vms` (list) and `POST /api/vms/{name}/{action}` (start/shutdown/suspend/resume/destroy).

- [ ] **Step 1: Add vmCmd to Commands slice in main()**

After `deployCmd()`:

```go
vmCmd(),
```

- [ ] **Step 2: Add vmCmd function**

```go
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
	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("VM action failed: %s", result.Error)
	}
	fmt.Printf("✔ VM %s: %s\n", name, action)
	return nil
}
```

- [ ] **Step 3: Build and verify**

```bash
cd /home/ogge/homelab-daemon && nix develop --command bash -c 'go build ./cmd/cli/'
./homelab vm --help
```

Expected: list/start/shutdown/suspend/resume subcommands shown.

- [ ] **Step 4: Commit**

```bash
git add cmd/cli/main.go && git commit -m "feat(cli): add vm list/start/shutdown/suspend/resume commands"
```

---

## Task 4: `homelab storage status`

**Files:**
- Modify: `cmd/cli/main.go`

Wraps `GET /api/storage`. Response is an array of pool objects.

- [ ] **Step 1: Check the storage API response shape**

```bash
grep -A 40 "GET /api/storage" /home/ogge/homelab-daemon/cmd/daemon/api.go | head -50
```

Note the JSON field names for the pool struct. You'll need them for the tabwriter output.

- [ ] **Step 2: Add storageCmd to Commands slice in main()**

After `vmCmd()`:

```go
storageCmd(),
```

- [ ] **Step 3: Add storageCmd function**

```go
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
	var pools []struct {
		UUID       string  `json:"uuid"`
		Name       string  `json:"name"`
		Mountdir   string  `json:"mountdir"`
		State      string  `json:"state"`
		UsedBytes  uint64  `json:"used_bytes"`
		TotalBytes uint64  `json:"total_bytes"`
		UsedPct    float64 `json:"used_pct"`
	}
	if err := json.Unmarshal(b, &pools); err != nil || len(pools) == 0 {
		fmt.Println(string(b))
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tMOUNT\tSTATE\tUSED\tTOTAL\tUSED%")
	for _, p := range pools {
		name := p.Name
		if name == "" {
			name = p.UUID[:8]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%.1f%%\n",
			name, p.Mountdir, p.State,
			fmtStorageBytes(p.UsedBytes), fmtStorageBytes(p.TotalBytes), p.UsedPct,
		)
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
```

**Note:** After adding this, check the actual JSON field names returned by `GET /api/storage` match what the struct above expects. Read `cmd/daemon/api.go` around line 646 to confirm field names before writing. Adjust struct tags if they differ.

- [ ] **Step 4: Build**

```bash
cd /home/ogge/homelab-daemon && nix develop --command bash -c 'go build ./cmd/cli/'
```

- [ ] **Step 5: Commit**

```bash
git add cmd/cli/main.go && git commit -m "feat(cli): add storage status command"
```

---

## Task 5: `homelab update` commands

**Files:**
- Modify: `cmd/cli/main.go`

Wraps `GET /api/updates` (list available container image updates) and `POST /api/updates/check` (trigger a fresh check). The daemon already has an update checker for podman containers.

- [ ] **Step 1: Check the updates API response shape**

```bash
grep -A 30 "GET /api/updates" /home/ogge/homelab-daemon/cmd/daemon/api.go | head -35
```

Note field names in the updates response.

- [ ] **Step 2: Add updateCmd to Commands slice**

After `storageCmd()`:

```go
updateCmd(),
```

- [ ] **Step 3: Add updateCmd function**

```go
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
					handleUpdateCheck()
					return nil
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
	var payload struct {
		Updates  map[string]struct {
			Unit        string `json:"unit"`
			Available   bool   `json:"available"`
			CurrentTag  string `json:"current_tag"`
			NewestTag   string `json:"newest_tag"`
		} `json:"updates"`
		Metadata struct {
			LastChecked string `json:"last_checked"`
			Checking    bool   `json:"checking"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		fmt.Println(string(b))
		return
	}
	if payload.Metadata.Checking {
		fmt.Println("⟳ Update check in progress...")
	} else if payload.Metadata.LastChecked != "" {
		fmt.Printf("Last checked: %s\n", payload.Metadata.LastChecked)
	}
	if len(payload.Updates) == 0 {
		fmt.Println("No updates available.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "UNIT\tCURRENT\tNEWEST\tAVAILABLE")
	for _, u := range payload.Updates {
		avail := "✗"
		if u.Available {
			avail = "✔"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", u.Unit, u.CurrentTag, u.NewestTag, avail)
	}
	w.Flush()
}

func handleUpdateCheck() {
	resp, err := httpClient.Post("http://unix/api/updates/check", "application/json", nil)
	if err != nil {
		die("contacting daemon: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		die("daemon returned status %d", resp.StatusCode)
	}
	fmt.Println("✔ Update check triggered.")
}
```

**Note:** Read `cmd/daemon/api.go` and `internal/updates/` to confirm the actual JSON shape of `GET /api/updates` before finalising the struct. Adjust field tags if they differ.

- [ ] **Step 4: Build**

```bash
cd /home/ogge/homelab-daemon && nix develop --command bash -c 'go build ./cmd/cli/'
```

- [ ] **Step 5: Commit**

```bash
git add cmd/cli/main.go && git commit -m "feat(cli): add update status/check commands"
```

---

## Task 6: `homelab notify test`

**Files:**
- Modify: `cmd/cli/main.go`

Sends a test SMTP notification using the daemon's notifier config to verify email delivery works. Reads config directly (same path as `doctor notify` — `/cache/appdata/homelab/services.yaml`).

- [ ] **Step 1: Add notifyCmd to Commands slice**

After `updateCmd()`:

```go
notifyCmd(),
```

- [ ] **Step 2: Add notifyCmd function**

```go
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

	// Synthesise a test report with one fake passing check.
	testReport := doctor.Report{
		Passed: 1,
		Failed: 0,
		Results: []doctor.Result{
			{Name: "test", OK: true, Detail: "This is a test notification from homelab CLI"},
		},
	}

	// Override Notify's no-op on zero failures by sending directly.
	hostname := cfg.Hostname
	if hostname == "" {
		hostname = "homelab"
	}
	subject := fmt.Sprintf("[homelab] Test notification from %s", hostname)
	body := doctor.FormatReportText(testReport)

	import_smtp_inline := func() error {
		// Use net/smtp directly since doctor.Notify skips on zero failures.
		addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
		auth := smtp_auth(cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
		msg := fmt.Sprintf(
			"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
			cfg.From, cfg.To, subject, body,
		)
		return smtp_send(addr, auth, cfg.From, strings.Split(cfg.To, ","), []byte(msg))
	}
	_ = import_smtp_inline // replaced below

	// Actual implementation — call net/smtp directly.
	_ = subject
	_ = body

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
	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
	if err := smtp.SendMail(addr, auth, cfg.From, strings.Split(cfg.To, ","), []byte(msg)); err != nil {
		return fmt.Errorf("send failed: %w", err)
	}
	fmt.Printf("✔ Test email sent to %s\n", cfg.To)
	return nil
}
```

**Important:** The `handleNotifyTest` function above has placeholder logic that won't compile. Replace it entirely with this clean version:

```go
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
```

Add `"net/smtp"` to imports if not already present (it already is via `internal/doctor` which uses it, but `cmd/cli/main.go` may need it directly for `sendTestEmail`).

- [ ] **Step 3: Build**

```bash
cd /home/ogge/homelab-daemon && nix develop --command bash -c 'go build ./cmd/cli/'
```

Fix any import or compilation errors.

- [ ] **Step 4: Commit**

```bash
git add cmd/cli/main.go && git commit -m "feat(cli): add notify test command for SMTP verification"
```

---

## Task 7: Final preflight + push

- [ ] **Step 1: Run full preflight**

```bash
cd /home/ogge/homelab-daemon && nix develop --command bash -c 'go vet ./... && go build ./cmd/daemon/ ./cmd/cli/'
```

Expected: clean, no errors.

- [ ] **Step 2: Run tests**

```bash
nix develop --command bash -c 'go test ./...'
```

Expected: all pass.

- [ ] **Step 3: Push**

```bash
git push
```

---

## Self-Review

**Spec coverage:**
- ✔ Shell completions (Task 1)
- ✔ `homelab deploy` / `rollback` (Task 2)
- ✔ `homelab vm list/start/shutdown/suspend/resume` (Task 3)
- ✔ `homelab storage status` (Task 4)
- ✔ `homelab update status/check` (Task 5)
- ✔ `homelab notify test` (Task 6)
- ✔ Final preflight + push (Task 7)

**Placeholder scan:**

Task 6 has a bad intermediate `handleNotifyTest` with a non-compiling `import_smtp_inline` closure — the plan immediately replaces it with the clean version. The implementer should skip the first version and write only the clean replacement directly. Marking this: **write only the clean `handleNotifyTest` and `sendTestEmail` pair from the "replace" block**.

Task 4 and 5 have "Note" callouts to verify JSON field names at implementation time — this is correct since the daemon API response shapes should be read directly rather than guessed.

**Type consistency:**
- `vmInfo` struct (Task 3) — used only in handleVMList ✔
- `doctor.NotifyConfig` / `doctor.LoadNotifyConfigFromFile` (Task 6) — defined in internal/doctor/notify.go Task 3 of prior plan ✔
- `doctor.FormatReportText` (Task 6) — defined same package ✔
- `fmtStorageBytes` (Task 4) — local helper, no collision with `fmtBytes` in internal/doctor ✔
