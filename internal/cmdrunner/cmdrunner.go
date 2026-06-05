package cmdrunner

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	logging "github.com/ogglord/homelab-logging"
)

type Escalation int

const (
	EscalateNone Escalation = iota
	EscalateAsUser
)

func (e Escalation) String() string {
	switch e {
	case EscalateNone:
		return "none"
	case EscalateAsUser:
		return "impersonate"
	default:
		return "unknown"
	}
}

type OutputMode int

const (
	OutputRaw OutputMode = iota
	OutputVerbose
	OutputCombined
)

type Builder struct {
	callerModule       string
	name               string
	args               []string
	user               string
	escalation         Escalation
	cwd                string
	env                []string
	ctx                context.Context
	timeout            time.Duration
	stdin              io.Reader
	lineHandler        func(stream, line string)
	outputMode         OutputMode
	secretEnvKeys      map[string]struct{}
	sandbox            bool
	sandboxAllowRead   []string
	sandboxAllowWrite  []string
	sandboxDenyRead    []string
	sandboxAllowDomains []string
}

type Result struct {
	Cmd       string
	Args      []string
	ExitCode  int
	Stdout    string
	Stderr    string
	Output    string
	Duration  time.Duration
	Cancelled bool
}

func (r Result) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("$ %s\n", strings.Join(r.Args, " ")))
	if r.Stdout != "" {
		sb.WriteString(r.Stdout)
		if !strings.HasSuffix(r.Stdout, "\n") {
			sb.WriteString("\n")
		}
	}
	if r.Stderr != "" {
		sb.WriteString("[stderr]\n")
		sb.WriteString(r.Stderr)
		if !strings.HasSuffix(r.Stderr, "\n") {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

type Error struct {
	Result Result
	Err    error
}

func (e *Error) Error() string {
	if e.Result.ExitCode > 0 {
		return fmt.Sprintf("command %s exited with code %d: %s", e.Result.Cmd, e.Result.ExitCode, strings.TrimSpace(e.Result.Stderr))
	}
	return fmt.Sprintf("command %s failed: %v", e.Result.Cmd, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

// Spawned is a handle to a long-running subprocess started via Builder.Spawn().
// The caller owns the pipes and must call Wait() to clean up.
type Spawned struct {
	Stdin   io.WriteCloser
	Stdout  io.ReadCloser
	Stderr  io.ReadCloser
	Pid     int

	cmd     *exec.Cmd
	builder *Builder
	start   time.Time
	result  Result
}

var canonicalModules = map[string]struct{}{
	"bug": {}, "api": {}, "secrets": {}, "monitor": {}, "middleware": {},
	"storage": {}, "updates": {}, "cmdrunner": {},
}

var cmdrunnerLog = logging.CmdLogger()

var forbiddenCommandNames = map[string]struct{}{
	"sudo":   {},
	"pkexec": {},
	"doas":   {},
	"su":     {},
	"sh":     {},
	"bash":   {},
}

// New creates a Builder for a single command. name is resolved to an
// absolute path via exec.LookPath. args are the command arguments.
// Callers that previously used "sh -c" should use .WithCwd(), .WithEnv(),
// and explicit args instead.
func New(callerModule, name string, args ...string) *Builder {
	if _, ok := canonicalModules[callerModule]; !ok {
		panic(fmt.Sprintf("cmdrunner: invalid caller module %q; must be canonical module", callerModule))
	}
	if name == "" {
		panic("cmdrunner: command name cannot be empty")
	}
	base := name
	if i := strings.LastIndex(name, "/"); i >= 0 {
		base = name[i+1:]
	}
	if _, forbidden := forbiddenCommandNames[base]; forbidden {
		panic(fmt.Sprintf(
			"cmdrunner: refusing %q as command name. Use .AsUser(<user>) for escalation. "+
				"Passing the escalation wrapper inline makes the cmd log line report "+
				"escalation=\"none\" even when it isn't.",
			name,
		))
	}
	return &Builder{
		callerModule: callerModule,
		name:         name,
		args:         args,
		escalation:   EscalateNone,
		outputMode:   OutputRaw,
	}
}

var forbiddenUIDs = map[int]struct{}{0: {}} // never impersonate root

func (b *Builder) AsUser(userName string) *Builder {
	u, err := user.Lookup(userName)
	if err != nil {
		panic(fmt.Sprintf("cmdrunner: AsUser(%q): user not found", userName))
	}
	uid, _ := strconv.Atoi(u.Uid)
	if _, forbidden := forbiddenUIDs[uid]; forbidden {
		panic(fmt.Sprintf("cmdrunner: AsUser(%q): forbidden — cannot impersonate root", userName))
	}
	b.user = userName
	b.escalation = EscalateAsUser
	return b
}

// nixosUserPath builds a PATH that mirrors the standard NixOS login
// environment: wrappers, user-local, nix profiles (home-manager, per-user
// NixOS, default), system path, and finally standard Unix fallbacks.
func nixosUserPath(homeDir, userName string) string {
	return fmt.Sprintf(
		"/run/wrappers/bin:%s/.local/bin:%s/.nix-profile/bin:/nix/profile/bin:"+
			"%s/.local/state/nix/profiles/profile/bin:/etc/profiles/per-user/%s/bin:"+
			"/nix/var/nix/profiles/default/bin:/run/current-system/sw/bin:/usr/bin:/bin",
		homeDir, homeDir, homeDir, userName,
	)
}

// resolveCommand finds name in the given PATH string. Uses exec.LookPath
// internally, temporarily swapping PATH so user-profile binaries are
// discoverable when AsUser is set.
var resolveCmdMu sync.Mutex

func resolveCommand(name, path string) (string, error) {
	resolveCmdMu.Lock()
	defer resolveCmdMu.Unlock()
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", path)
	defer os.Setenv("PATH", oldPath)
	return exec.LookPath(name)
}

func (b *Builder) WithCwd(dir string) *Builder {
	b.cwd = dir
	return b
}

func (b *Builder) WithEnv(kvs ...string) *Builder {
	b.env = append(b.env, kvs...)
	return b
}

type SecretResolver func(name string) string

var secretResolver SecretResolver

func SetSecretResolver(fn SecretResolver) { secretResolver = fn }

// WithSecret injects named secrets as KEY=value env vars. Missing or
// empty secrets are silently skipped with a warning (no panic). Use
// RequireSecret when the secret must be present.
func (b *Builder) WithSecret(names ...string) *Builder {
	if secretResolver == nil {
		return b
	}
	if b.secretEnvKeys == nil {
		b.secretEnvKeys = map[string]struct{}{}
	}
	for _, name := range names {
		kv := secretResolver(name)
		if kv == "" {
			cmdrunnerLog.Warn("secret not found in registry",
				"name", name, "caller_module", b.callerModule)
			continue
		}
		if i := strings.IndexByte(kv, '='); i > 0 {
			b.secretEnvKeys[kv[:i]] = struct{}{}
		}
		b.env = append(b.env, kv)
	}
	return b
}

// RequireSecret injects a named secret as a KEY=value env var, panicking
// if the secret is missing, empty, or the resolver returns nothing.
func (b *Builder) RequireSecret(names ...string) *Builder {
	if secretResolver == nil {
		panic(fmt.Sprintf(
			"cmdrunner: secret resolver not set; module %q called RequireSecret before init",
			b.callerModule,
		))
	}
	if b.secretEnvKeys == nil {
		b.secretEnvKeys = map[string]struct{}{}
	}
	for _, name := range names {
		kv := secretResolver(name)
		if kv == "" {
			panic(fmt.Sprintf(
				"cmdrunner: required secret %q requested by module %q is not in the registry; "+
					"declare it in modules/secrets.nix and rebuild",
				name, b.callerModule,
			))
		}
		if i := strings.IndexByte(kv, '='); i > 0 {
			b.secretEnvKeys[kv[:i]] = struct{}{}
		}
		b.env = append(b.env, kv)
	}
	return b
}

func (b *Builder) WithContext(ctx context.Context) *Builder {
	b.ctx = ctx
	return b
}

func (b *Builder) WithTimeout(d time.Duration) *Builder {
	b.timeout = d
	return b
}

func (b *Builder) WithStdin(r io.Reader) *Builder {
	b.stdin = r
	return b
}

func (b *Builder) WithLineHandler(fn func(stream, line string)) *Builder {
	b.lineHandler = fn
	return b
}

func (b *Builder) Output(mode OutputMode) *Builder {
	b.outputMode = mode
	return b
}

// InSandbox enables sandbox-runtime wrapping via srt for this command.
// Defaults: allow read everywhere, deny write everywhere except CWD and
// /tmp, deny read to ~/.ssh ~/.aws /run/secrets /cache/secrets /etc/ssh
// /var/lib/sops-age, allow network to github.com + api.github.com. Use
// .AllowWrite(), .DenyRead(), .AllowNetwork() to adjust.
func (b *Builder) InSandbox() *Builder {
	b.sandbox = true
	b.sandboxAllowRead = []string{}
	b.sandboxAllowWrite = []string{".", "/tmp"}
	b.sandboxDenyRead = []string{"~/.ssh", "~/.aws", "/run", "/cache", "/etc/ssh", "/var/lib/sops-age"}
	b.sandboxAllowDomains = []string{"github.com", "*.github.com", "api.github.com", "raw.githubusercontent.com"}
	return b
}

// AllowWrite adds write-permitted paths for the sandbox.
func (b *Builder) AllowWrite(paths ...string) *Builder {
	b.sandboxAllowWrite = append(b.sandboxAllowWrite, paths...)
	return b
}

// AllowRead adds read-permitted paths for the sandbox. These paths are
// re-allowed within regions denied by denyRead via srt's allowRead setting
// (ro-bind over tmpfs). Use when a broad denyRead mounts tmpfs over a
// directory whose subtree must remain partially accessible — e.g.
// denyRead: [/run] + allowRead: [/run/current-system] so pi can exec bash.
func (b *Builder) AllowRead(paths ...string) *Builder {
	b.sandboxAllowRead = append(b.sandboxAllowRead, paths...)
	return b
}

// DenyRead adds read-denied paths for the sandbox.
func (b *Builder) DenyRead(paths ...string) *Builder {
	b.sandboxDenyRead = append(b.sandboxDenyRead, paths...)
	return b
}

// AllowNetwork restricts network access to the given domains.
func (b *Builder) AllowNetwork(domains ...string) *Builder {
	b.sandboxAllowDomains = append(b.sandboxAllowDomains, domains...)
	return b
}

func (b *Builder) buildCredential() (*syscall.Credential, error) {
	u, err := user.Lookup(b.user)
	if err != nil {
		return nil, fmt.Errorf("lookup user %q: %w", b.user, err)
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	gidStrs, _ := u.GroupIds()
	gids := make([]uint32, len(gidStrs))
	for i, g := range gidStrs {
		v, _ := strconv.Atoi(g)
		gids[i] = uint32(v)
	}
	return &syscall.Credential{
		Uid:    uint32(uid),
		Gid:    uint32(gid),
		Groups: gids,
	}, nil
}

// build prepares the exec.Cmd with credential, env, cwd, and sandbox wrapping.
func (b *Builder) build() (*exec.Cmd, error) {
	// Resolve absolute path. When impersonating a user, search their NixOS
	// profile paths so user-installed binaries (home-manager, nix profile)
	// are discoverable, not just system-wide ones.
	var absCmd string
	var err error
	if b.escalation == EscalateAsUser && b.user != "" {
		u, _ := user.Lookup(b.user)
		absCmd, err = resolveCommand(b.name, nixosUserPath(u.HomeDir, u.Username))
	} else {
		absCmd, err = exec.LookPath(b.name)
	}
	if err != nil {
		return nil, fmt.Errorf("command %q not found in PATH: %w", b.name, err)
	}

	cmdName := absCmd
	cmdArgs := b.args

	// Sandbox wrapping: prefix with srt.
	if b.sandbox {
		srtPath, err := exec.LookPath("srt")
		if err != nil {
			return nil, fmt.Errorf("sandbox-runtime (srt) not found in PATH: %w", err)
		}
		srtSettings, err := b.writeTempSandboxSettings()
		if err != nil {
			return nil, fmt.Errorf("sandbox settings: %w", err)
		}
		cmdArgs = append([]string{"--settings", srtSettings, "--", cmdName}, cmdArgs...)
		cmdName = srtPath
	}

	// Credential for impersonation.
	var cred *syscall.Credential
	if b.escalation == EscalateAsUser && b.user != "" {
		cred, err = b.buildCredential()
		if err != nil {
			return nil, fmt.Errorf("credential for user %q: %w", b.user, err)
		}
	}

	ctx := b.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if b.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, b.timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	if b.cwd != "" {
		cmd.Dir = b.cwd
	}
	if cred != nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: cred}
	}
	if b.stdin != nil {
		cmd.Stdin = b.stdin
	}

	// Build environment.
	if b.escalation == EscalateAsUser && b.user != "" {
		u, _ := user.Lookup(b.user)
		env := []string{
			"PATH=" + nixosUserPath(u.HomeDir, u.Username),
			"HOME=" + u.HomeDir,
			"USER=" + u.Username,
			"LOGNAME=" + u.Username,
		}
		env = append(env, b.env...)
		cmd.Env = env
	} else if len(b.env) > 0 {
		cmd.Env = append(os.Environ(), b.env...)
	}

	return cmd, nil
}

// Spawn starts a long-running process and returns handles for bidirectional I/O.
// The caller MUST call Wait() on the returned Spawned to emit the log line.
func (b *Builder) Spawn() (*Spawned, error) {
	cmd, err := b.build()
	if err != nil {
		return nil, err
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("start process: %w", err)
	}

	return &Spawned{
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
		Pid:     cmd.Process.Pid,
		cmd:     cmd,
		builder: b,
		start:   time.Now(),
	}, nil
}

// Wait blocks until the process exits and emits the structured log line.
func (s *Spawned) Wait() (Result, error) {
	err := s.cmd.Wait()
	s.result.ExitCode = -1
	s.result.Duration = time.Since(s.start)
	s.result.Cmd = s.builder.name
	s.result.Args = append([]string{s.builder.name}, s.builder.args...)
	if s.builder.sandbox {
		s.result.Args = append([]string{"srt", "--settings", "<temp>", "--", s.builder.name}, s.builder.args...)
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				s.result.ExitCode = status.ExitStatus()
			} else {
				s.result.ExitCode = exitErr.ExitCode()
			}
		}
	}
	if s.result.ExitCode == -1 && err == nil {
		s.result.ExitCode = 0
	}
	if s.builder.ctx != nil && s.builder.ctx.Err() != nil {
		s.result.Cancelled = true
	}

	s.builder.logResult(s.result, s.start, err)
	return s.result, nil
}

// Kill sends SIGTERM then SIGKILL after a grace period.
func (s *Spawned) Kill() {
	_ = s.cmd.Process.Signal(syscall.SIGTERM)
	time.Sleep(2 * time.Second)
	_ = s.cmd.Process.Signal(syscall.SIGKILL)
}

func (b *Builder) Run() (Result, error) {
	start := time.Now()

	// Resolve absolute path. When impersonating, search the user's NixOS
	// profile paths so home-manager / nix-profile binaries are found.
	var absCmd string
	var err error
	if b.escalation == EscalateAsUser && b.user != "" {
		u, _ := user.Lookup(b.user)
		absCmd, err = resolveCommand(b.name, nixosUserPath(u.HomeDir, u.Username))
	} else {
		absCmd, err = exec.LookPath(b.name)
	}
	if err != nil {
		return b.logAndError(Result{ExitCode: -1}, start,
			fmt.Errorf("command %q not found in PATH: %w", b.name, err), b.ctx)
	}

	var cred *syscall.Credential
	if b.escalation == EscalateAsUser && b.user != "" {
		cred, err = b.buildCredential()
		if err != nil {
			return b.logAndError(Result{ExitCode: -1}, start,
				fmt.Errorf("credential for user %q: %w", b.user, err), b.ctx)
		}
	}

	res := Result{
		Cmd:      absCmd,
		Args:     append([]string{absCmd}, b.args...),
		ExitCode: -1,
	}

	ctx := b.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	var cancel context.CancelFunc
	if b.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, b.timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, absCmd, b.args...)
	if b.cwd != "" {
		cmd.Dir = b.cwd
	}
	if cred != nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: cred}
	}
	if b.stdin != nil {
		cmd.Stdin = b.stdin
	}

	// Build clean environment for impersonated processes.
	if b.escalation == EscalateAsUser && b.user != "" {
		u, _ := user.Lookup(b.user)
		env := []string{
			"PATH=" + nixosUserPath(u.HomeDir, u.Username),
			"HOME=" + u.HomeDir,
			"USER=" + u.Username,
			"LOGNAME=" + u.Username,
		}
		env = append(env, b.env...)
		cmd.Env = env
	} else if len(b.env) > 0 {
		cmd.Env = append(os.Environ(), b.env...)
	}

	// Pipes
	var stdoutPipe, stderrPipe io.ReadCloser
	var stdoutBuf, stderrBuf strings.Builder

	if b.lineHandler != nil || b.outputMode == OutputCombined {
		stdoutPipe, err = cmd.StdoutPipe()
		if err != nil {
			return b.logAndError(res, start, fmt.Errorf("stdout pipe: %w", err), ctx)
		}
		stderrPipe, err = cmd.StderrPipe()
		if err != nil {
			_ = stdoutPipe.Close()
			return b.logAndError(res, start, fmt.Errorf("stderr pipe: %w", err), ctx)
		}
	} else {
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
	}

	if err := cmd.Start(); err != nil {
		return b.logAndError(res, start, fmt.Errorf("start process: %w", err), ctx)
	}

	if stdoutPipe != nil && stderrPipe != nil {
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(stdoutPipe)
			for scanner.Scan() {
				line := scanner.Text()
				if b.lineHandler != nil {
					b.lineHandler("stdout", line)
				}
				stdoutBuf.WriteString(line + "\n")
			}
		}()

		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(stderrPipe)
			for scanner.Scan() {
				line := scanner.Text()
				if b.lineHandler != nil {
					b.lineHandler("stderr", line)
				}
				if b.outputMode == OutputCombined {
					stdoutBuf.WriteString(line + "\n")
				} else {
					stderrBuf.WriteString(line + "\n")
				}
			}
		}()

		wg.Wait()
	}

	err = cmd.Wait()
	res.Duration = time.Since(start)
	res.Stdout = stdoutBuf.String()
	res.Stderr = stderrBuf.String()

	if ctx.Err() != nil {
		res.Cancelled = true
	}

	switch b.outputMode {
	case OutputVerbose:
		res.Output = res.String()
	case OutputCombined:
		res.Output = res.Stdout
	default:
		res.Output = res.Stdout
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				res.ExitCode = status.ExitStatus()
			} else {
				res.ExitCode = exitErr.ExitCode()
			}
		}
		return b.logAndError(res, start, err, ctx)
	}

	res.ExitCode = 0
	b.logResult(res, start, nil)
	return res, nil
}

func (b *Builder) logAndError(res Result, start time.Time, err error, ctx context.Context) (Result, error) {
	res.Duration = time.Since(start)
	if ctx != nil && ctx.Err() != nil {
		res.Cancelled = true
	}
	b.logResult(res, start, err)
	return res, &Error{Result: res, Err: err}
}

func (b *Builder) summary() string {
	base := b.name
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	full := base
	if len(b.args) > 0 {
		full = base + " " + strings.Join(b.args, " ")
	}
	const maxLen = 100
	if len(full) > maxLen {
		full = full[:maxLen-1] + "…"
	}
	return full
}

func (b *Builder) logResult(res Result, start time.Time, err error) {
	durMs := int(res.Duration.Milliseconds())
	logger := logging.CmdLogger()
	cmdSummary := b.summary()

	args := res.Args
	if len(b.secretEnvKeys) > 0 {
		args = make([]string, len(res.Args))
		for i, a := range res.Args {
			if eq := strings.IndexByte(a, '='); eq > 0 {
				if _, redact := b.secretEnvKeys[a[:eq]]; redact {
					args[i] = a[:eq] + "=<redacted>"
					continue
				}
			}
			args[i] = a
		}
	}

	if err != nil {
		if res.ExitCode > 0 {
			logger.Warn(
				fmt.Sprintf("%s → exit %d (%dms)", cmdSummary, res.ExitCode, durMs),
				"cmd", res.Cmd,
				"args", args,
				"escalation", b.escalation.String(),
				"exit_code", res.ExitCode,
				"duration_ms", durMs,
				"cancelled", res.Cancelled,
				"caller_module", b.callerModule,
				"error", err.Error(),
			)
		} else {
			logger.Error(
				fmt.Sprintf("%s → exec failed: %v", cmdSummary, err),
				"cmd", res.Cmd,
				"args", args,
				"escalation", b.escalation.String(),
				"exit_code", res.ExitCode,
				"duration_ms", durMs,
				"cancelled", res.Cancelled,
				"caller_module", b.callerModule,
				"error", err.Error(),
			)
		}
	} else {
		logger.Info(
			fmt.Sprintf("%s → ok (%dms)", cmdSummary, durMs),
			"cmd", res.Cmd,
			"args", args,
			"escalation", b.escalation.String(),
			"exit_code", res.ExitCode,
			"duration_ms", durMs,
			"cancelled", res.Cancelled,
			"caller_module", b.callerModule,
		)
	}
}

// writeTempSandboxSettings generates a .srt-settings.json in /tmp and
// returns the path.
func (b *Builder) writeTempSandboxSettings() (string, error) {
	// Filter denyRead to only include paths that actually exist. bwrap
	// (used by srt) mounts tmpfs over each denied path; if the path
	// doesn't exist in the sandbox root, the mount fails.
	filteredDeny := make([]string, 0, len(b.sandboxDenyRead))
	for _, p := range b.sandboxDenyRead {
		// Expand ~ to the impersonated user's home; fall back to root.
		expanded := p
		if strings.HasPrefix(p, "~/") {
			if b.user != "" {
				if u, err := user.Lookup(b.user); err == nil {
					expanded = u.HomeDir + p[1:]
				}
			} else {
				expanded = os.Getenv("HOME") + p[1:]
			}
		}
		if _, err := os.Stat(expanded); os.IsNotExist(err) {
			cmdrunnerLog.Warn("sandbox: skipping non-existent denyRead path",
				"path", p, "expanded", expanded, "caller_module", b.callerModule)
			continue
		}
		filteredDeny = append(filteredDeny, p)
	}

	settings := map[string]any{
		"network": map[string]any{
			"allowedDomains":      b.sandboxAllowDomains,
			"deniedDomains":       []string{},
			"allowUnixSockets":    []string{},
			"allowAllUnixSockets": false,
			"allowLocalBinding":   false,
		},
		"filesystem": map[string]any{
			"denyRead":  filteredDeny,
			"allowRead": b.sandboxAllowRead,
			"allowWrite": b.sandboxAllowWrite,
			"denyWrite":  []string{},
		},
		"ignoreViolations": map[string]any{
			"git": []string{"/usr/bin/ssh"},
			"nix": []string{"/nix/store"},
		},
		"enableWeakerNestedSandbox":    false,
		"enableWeakerNetworkIsolation": false,
	}

	f, err := os.CreateTemp("/tmp", "cmdrunner-sandbox-*.json")
	if err != nil {
		return "", err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(settings); err != nil {
		os.Remove(f.Name()) //nolint:errcheck
		return "", err
	}

	// Make world-readable so the impersonated user (e.g. ogge) can read
	// the settings when srt --settings <path> runs under their uid.
	if err := os.Chmod(f.Name(), 0644); err != nil {
		os.Remove(f.Name()) //nolint:errcheck
		return "", err
	}
	return f.Name(), nil
}
