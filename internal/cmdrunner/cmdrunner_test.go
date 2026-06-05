package cmdrunner

import (
	"bufio"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestEchoSuccess(t *testing.T) {
	res, err := New("api", "echo", "hello").Run()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got: %d", res.ExitCode)
	}
	trimmed := strings.TrimSpace(res.Stdout)
	if trimmed != "hello" {
		t.Errorf("expected stdout 'hello', got: %q", res.Stdout)
	}
	if res.Output != res.Stdout {
		t.Errorf("expected Output to match Stdout under OutputRaw, got %q", res.Output)
	}
}

func TestCommandFailed(t *testing.T) {
	_, err := New("api", "false").Run()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cmdErr *Error
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected error of type *cmdrunner.Error, got: %T", err)
	}

	if cmdErr.Result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got: %d", cmdErr.Result.ExitCode)
	}
}

func TestWithLineHandler(t *testing.T) {
	var lines []string
	res, err := New("api", "echo", "hello\nworld").
		WithLineHandler(func(stream, line string) {
			lines = append(lines, line)
		}).
		Run()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got: %v", lines)
	}

	if lines[0] != "hello" || lines[1] != "world" {
		t.Errorf("expected hello/world, got: %v", lines)
	}

	trimmedStdout := strings.TrimSpace(res.Stdout)
	if trimmedStdout != "hello\nworld" {
		t.Errorf("expected res.Stdout 'hello\\nworld', got %q", res.Stdout)
	}
}

func TestTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := New("api", "sleep", "1").
		WithContext(ctx).
		Run()

	if err == nil {
		t.Fatal("expected error from timeout, got nil")
	}

	var cmdErr *Error
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected error to wrap *cmdrunner.Error, got %T", err)
	}

	if !cmdErr.Result.Cancelled {
		t.Errorf("expected Result.Cancelled to be true, got false")
	}
}

func TestEscalationStructure(t *testing.T) {
	// We cannot easily test active escalation executions in a non-root developer environment
	// without actual sudo privileges, but we can verify the prepended arguments logic.
	b := New("api", "git", "status").AsUser("ogge")
	if b.escalation != EscalateAsUser || b.user != "ogge" {
		t.Errorf("expected EscalateAsUser and user 'ogge', got: %v %q", b.escalation, b.user)
	}
}

// TestRejectsInlineEscalation ensures New() panics when a caller tries to
// pass sudo/pkexec/doas/su as the command name instead of using the
// builder methods.
func TestRejectsInlineEscalation(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
	}{
		{"sudo bare", "sudo"},
		{"sudo absolute", "/run/wrappers/bin/sudo"},
		{"sudo /usr/bin", "/usr/bin/sudo"},
		{"pkexec bare", "pkexec"},
		{"pkexec absolute", "/run/wrappers/bin/pkexec"},
		{"doas", "doas"},
		{"su", "/usr/bin/su"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("expected panic for %q, got none", tc.cmd)
				}
				msg, _ := r.(string)
				if !strings.Contains(msg, "AsUser") {
					t.Errorf("panic message should mention .AsUser(), got: %q", msg)
				}
			}()
			_ = New("api", tc.cmd, "-i", "-u", "ogge", "echo", "hi")
		})
	}
}

// TestAllowsLegitimateCommands sanity-checks the forbidden list doesn't
// reject normal commands that happen to share substrings.
func TestAllowsLegitimateCommands(t *testing.T) {
	for _, cmd := range []string{"git", "echo", "systemctl", "bcachefs", "sudoers-thing-not-real"} {
		// Should not panic.
		_ = New("api", cmd)
	}
}

func TestRejectsShellAsCommand(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for 'sh', got none")
		}
	}()
	_ = New("api", "sh", "-c", "echo hi")
}

func TestBuildAccumulatesFields(t *testing.T) {
	b := New("api", "echo", "hello").
		AsUser("ogge").
		WithCwd("/tmp").
		WithEnv("FOO=bar", "BAZ=qux").
		InSandbox().
		AllowWrite("/boot").
		DenyRead("/test-deny").
		AllowNetwork("example.com")

	if !b.sandbox {
		t.Error("expected sandbox to be enabled")
	}
	if b.escalation != EscalateAsUser {
		t.Errorf("expected EscalateAsUser, got %v", b.escalation)
	}
	if b.cwd != "/tmp" {
		t.Errorf("expected cwd '/tmp', got %q", b.cwd)
	}
	if len(b.sandboxAllowWrite) != 3 { // default ".", "/tmp" + "/boot"
		t.Errorf("expected 3 allowWrite entries, got %d: %v", len(b.sandboxAllowWrite), b.sandboxAllowWrite)
	}
	if len(b.sandboxDenyRead) != 7 { // 6 defaults + "/test-deny"
		t.Errorf("expected 7 denyRead entries, got %d", len(b.sandboxDenyRead))
	}
	if len(b.sandboxAllowDomains) != 5 || b.sandboxAllowDomains[4] != "example.com" {
		// 4 defaults + example.com
		t.Errorf("expected 5 allowDomains with example.com last, got %v", b.sandboxAllowDomains)
	}
}

func TestSandboxSettingsFile(t *testing.T) {
	b := New("api", "echo", "hi").
		InSandbox().
		AllowWrite("/custom").
		DenyRead("/run/secrets", "~/.ssh").
		AllowNetwork("example.com", "*.example.com")

	path, err := b.writeTempSandboxSettings()
	if err != nil {
		t.Fatalf("writeTempSandboxSettings: %v", err)
	}
	defer deleteTempFile(t, path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings file: %v", err)
	}
	content := string(data)

	// Verify key settings are present.
	for _, want := range []string{
		"\"allowedDomains\"",
		"example.com",
		"*.example.com",
		"\"denyRead\"",
		"/run/secrets",
		"~/.ssh",
		"\"allowWrite\"",
		".",
		"/tmp",
		"/custom",
		"\"enableWeakerNestedSandbox\"",
		"\"ignoreViolations\"",
		"git",
		"/usr/bin/ssh",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("settings file missing %q", want)
		}
	}
}

func TestSpawnEcho(t *testing.T) {
	spawn, err := New("api", "echo", "hello spawn").Spawn()
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	var buf strings.Builder
	scanner := bufio.NewScanner(spawn.Stdout)
	for scanner.Scan() {
		buf.WriteString(scanner.Text() + "\n")
	}
	spawn.Stdout.Close()

	res, err := spawn.Wait()
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit code %d, want 0", res.ExitCode)
	}
	if strings.TrimSpace(buf.String()) != "hello spawn" {
		t.Errorf("got %q, want 'hello spawn'", strings.TrimSpace(buf.String()))
	}
}

func TestSpawnKill(t *testing.T) {
	spawn, err := New("api", "sleep", "10").Spawn()
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	spawn.Kill()
	res, _ := spawn.Wait()
	if res.ExitCode != -1 {
		t.Errorf("exit code %d, want -1 (killed)", res.ExitCode)
	}
}

func TestCommandNotFound(t *testing.T) {
	_, err := New("api", "nonexistent-command-zzz123").Run()
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("error should mention PATH, got: %v", err)
	}
}

func deleteTempFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Logf("failed to remove temp file %s: %v", path, err)
	}
}
