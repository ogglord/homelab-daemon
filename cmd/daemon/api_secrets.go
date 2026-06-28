package main

// api_secrets.go — secrets management API for homelab-daemon.
//
// Routes:
//   GET  /api/secrets         — list all declared secrets with present/missing status
//   GET  /api/secrets/:name   — single secret metadata
//   PUT  /api/secrets/:name   — set/rotate a secret value (sops set + git + deploy-pending flag)
//   DELETE /api/secrets/:name — wipe a secret value (set to empty string)
//   POST /api/secrets/deploy  — run nh os switch; streams progress via SSE
//
// The canonical list of secrets is read from /etc/homelab-secrets-registry.json,
// written at activation by modules/secrets.nix.
//
// Secret values are NEVER returned by the API — only metadata.
// The deploy-pending flag is a simple file at /run/homelab-daemon/secrets-pending.
// It is set after any successful PUT/DELETE and cleared after a successful switch.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	api "github.com/ogglord/homelab-api"
	"github.com/ogglord/homelab-daemon/internal/cmdrunner"
	logging "github.com/ogglord/homelab-logging"
)

func init() {
	cmdrunner.SetSecretResolver(func(name string) string {
		entries, err := loadSecretsRegistry()
		if err != nil {
			return ""
		}
		for _, e := range entries {
			// 1. Exact match (service/VAR_NAME).
			if e.Name == name {
				return readSecretKV(e, name)
			}
			// 2. Trailing-component match (just VAR_NAME).
			if parts := strings.SplitN(e.Name, "/", 2); len(parts) == 2 && parts[1] == name {
				return readSecretKV(e, name)
			}
		}
		return ""
	})
}

func readSecretKV(e api.SecretEntry, alias string) string {
	data, err := os.ReadFile(e.RunPath)
	if err != nil {
		return ""
	}
	kv := strings.TrimSpace(string(data))
	if !strings.Contains(kv, "=") {
		kv = alias + "=" + kv
	}
	return kv
}

const (
	secretsRegistryPath = "/etc/homelab-secrets-registry.json"
	sopsBin             = "sops"
	ageKeyFile          = "/var/lib/sops-age/keys.txt"
	deployPendingPath   = "/run/homelab-daemon/secrets-pending"
	// timestampStorePath persists per-secret rotation times across deploys.
	// Lives in StateDirectory so it survives nh os switch (unlike /run).
	timestampStorePath = "/var/lib/homelab-daemon/secret-timestamps.json"
)

// resolveRepoPath returns the flake root directory from NH_FLAKE
// (e.g. "/home/ogge/repos/nixos" or "/path/to/flake#host"),
// falling back to "." if the env var is absent.
var resolveRepoPath = func() string {
	nhFlake := os.Getenv("NH_FLAKE")
	if nhFlake == "" {
		nhFlake = os.Getenv("FLAKE")
	}
	if nhFlake == "" {
		secretsLog.Warn("NH_FLAKE not set; falling back to CWD for sops operations")
		return "."
	}
	// Strip the flake attr suffix: /path/to/flake#host → /path/to/flake
	if idx := strings.LastIndexByte(nhFlake, '#'); idx >= 0 {
		nhFlake = nhFlake[:idx]
	}
	if nhFlake == "" {
		return "."
	}
	return nhFlake
}()

var (
	// secretsYamlPath is the fully-qualified path to secrets.yaml (derived from NH_FLAKE).
	secretsYamlPath = resolveRepoPath + "/secrets/secrets.yaml"
	// sopsConfigPath is the sops config file in the flake root.
	sopsConfigPath = resolveRepoPath + "/.sops.yaml"
)

// SecretEntry and SecretStatus moved to pkg/api (the shared wire
// contract). Local aliases keep call-sites in this file readable.
type SecretEntry = api.SecretEntry
type SecretStatus = api.SecretStatus

var secretsLog = logging.Logger("secrets")

func loadSecretsRegistry() ([]SecretEntry, error) {
	data, err := os.ReadFile(secretsRegistryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // registry not written yet (pre-first-deploy)
		}
		return nil, err
	}
	var entries []SecretEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse secrets registry: %w", err)
	}
	return entries, nil
}

func loadTimestamps() map[string]string {
	data, err := os.ReadFile(timestampStorePath)
	if err != nil {
		return map[string]string{}
	}
	var ts map[string]string
	if err := json.Unmarshal(data, &ts); err != nil {
		return map[string]string{}
	}
	return ts
}

func recordTimestamp(name string) {
	ts := loadTimestamps()
	ts[name] = time.Now().UTC().Format(time.RFC3339)
	data, _ := json.Marshal(ts)
	_ = os.WriteFile(timestampStorePath, data, 0o600)
}

func secretStatus(entry SecretEntry) SecretStatus {
	s := SecretStatus{SecretEntry: entry}
	info, err := os.Stat(entry.RunPath)
	if err == nil && info.Size() > 0 {
		s.Present = true
		// Read first 3 bytes for the preview; never expose more
		if f, err := os.Open(entry.RunPath); err == nil {
			buf := make([]byte, 3)
			n, _ := f.Read(buf)
			f.Close()
			if n > 0 {
				s.Preview = string(buf[:n]) + "•••"
			}
		}
	}
	return s
}

// enrichWithTimestamps annotates a slice of SecretStatus with rotation times
// from the sidecar store (immune to nh os switch rewriting /run/secrets).
func enrichWithTimestamps(statuses []SecretStatus) {
	ts := loadTimestamps()
	for i, s := range statuses {
		if t, ok := ts[s.Name]; ok {
			statuses[i].ModifiedAt = t
		}
	}
}

func isDeployPending() bool {
	_, err := os.Stat(deployPendingPath)
	return err == nil
}

func setDeployPending(pending bool) {
	if pending {
		_ = os.WriteFile(deployPendingPath, []byte("1"), 0o600)
	} else {
		_ = os.Remove(deployPendingPath)
	}
}

// sopsSet updates a single key in secrets/secrets.yaml using `sops set`.
// keyPath is the YAML dotted path, e.g. "caddy.CLOUDFLARE_API_TOKEN".
// The value is written to sops via stdin.
func sopsSet(keyPath, value string) error {
	// Convert service/VAR_NAME → ["service"]["VAR_NAME"] for sops JSON path syntax.
	parts := strings.SplitN(keyPath, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid secret name %q: expected service/VAR_NAME", keyPath)
	}
	jsonPath := fmt.Sprintf(`["%s"]["%s"]`, parts[0], parts[1])

	res, err := cmdrunner.New("secrets", sopsBin, "--config", sopsConfigPath, "set", "--value-stdin", secretsYamlPath, jsonPath).
		WithEnv("SOPS_AGE_KEY_FILE=" + ageKeyFile).
		WithStdin(strings.NewReader(fmt.Sprintf("%q", value))).
		Output(cmdrunner.OutputCombined).
		Run()
	if err != nil {
		return fmt.Errorf("sops set failed: %w\noutput: %s", err, res.Output)
	}
	// Restore ownership to ogge since sops runs as root and recreates the file
	_, _ = cmdrunner.New("secrets", "chown", "ogge:users", secretsYamlPath).Run()
	return nil
}

func gitCommitAndPush(message string) error {
	for _, args := range [][]string{
		{"add", secretsYamlPath},
		{"commit", "-m", message},
		{"push"},
	} {
		// Run git as the 'ogge' user so it uses the correct SSH keys.
		// Use -C <repo> instead of WithCwd because AsUser uses sudo -i which
		// would chdir to ogge's home before exec, defeating the cwd.
		gitArgs := append([]string{"-C", resolveRepoPath}, args...)
		res, err := cmdrunner.New("secrets", "git", gitArgs...).
			AsUser("ogge").
			Output(cmdrunner.OutputCombined).
			Run()
		if err != nil {
			return fmt.Errorf("git %v failed: %w\noutput: %s", args, err, res.Output)
		}
	}
	return nil
}

func registerSecretsAPI(mux *http.ServeMux) {
	// GET /api/secrets — list all secrets with present/missing status
	mux.HandleFunc("GET /api/secrets", func(w http.ResponseWriter, r *http.Request) {
		entries, err := loadSecretsRegistry()
		if err != nil {
			secretsLog.Error("secrets registry load failed", "error", err)
			http.Error(w, `{"error":"failed to load secrets registry"}`, http.StatusInternalServerError)
			return
		}

		var out []SecretStatus
		for _, e := range entries {
			out = append(out, secretStatus(e))
		}
		if out == nil {
			out = []SecretStatus{}
		}
		enrichWithTimestamps(out)

		resp := map[string]any{
			"secrets":        out,
			"deploy_pending": isDeployPending(),
		}
		writeJSON(w, resp)
	})

	// GET /api/secrets/:name — single secret metadata
	mux.HandleFunc("GET /api/secrets/{name...}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		entries, err := loadSecretsRegistry()
		if err != nil {
			http.Error(w, `{"error":"failed to load secrets registry"}`, http.StatusInternalServerError)
			return
		}
		for _, e := range entries {
			if e.Name == name {
				writeJSON(w, secretStatus(e))
				return
			}
		}
		http.Error(w, `{"error":"secret not found"}`, http.StatusNotFound)
	})

	// PUT /api/secrets/:name — set/rotate a secret value
	mux.HandleFunc("PUT /api/secrets/{name...}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")

		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
			return
		}
		if body.Value == "" {
			http.Error(w, `{"error":"value must not be empty"}`, http.StatusBadRequest)
			return
		}

		secretsLog.Info("rotating secret", "name", name)
		if err := sopsSet(name, body.Value); err != nil {
			secretsLog.Error("sops set failed", "name", name, "error", err)
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}

		msg := fmt.Sprintf("secrets: rotate %s via dashboard", name)
		if err := gitCommitAndPush(msg); err != nil {
			secretsLog.Error("git commit/push failed", "error", err)
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}

		recordTimestamp(name)
		setDeployPending(true)
		secretsLog.Info("secret rotated, deploy pending", "name", name)
		writeJSON(w, map[string]any{"success": true, "deploy_pending": true})
	})

	// DELETE /api/secrets/:name — wipe a secret (set to empty string)
	mux.HandleFunc("DELETE /api/secrets/{name...}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		secretsLog.Info("wiping secret", "name", name)

		if err := sopsSet(name, ""); err != nil {
			secretsLog.Error("sops set (wipe) failed", "name", name, "error", err)
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}

		msg := fmt.Sprintf("secrets: wipe %s via dashboard", name)
		if err := gitCommitAndPush(msg); err != nil {
			secretsLog.Error("git commit/push failed", "error", err)
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}

		setDeployPending(true)
		writeJSON(w, map[string]any{"success": true, "deploy_pending": true})
	})

	// POST /api/secrets/deploy — run nh os switch, stream progress via SSE
	mux.HandleFunc("POST /api/secrets/deploy", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		send := func(msg string) {
			data, _ := json.Marshal(map[string]string{"type": "log", "message": msg})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		send("Starting nh os switch...")
		secretsLog.Info("secrets deploy triggered via dashboard")

		_, err := cmdrunner.New("secrets", "nh", "os", "switch").
			WithCwd(resolveRepoPath).
			WithContext(r.Context()).
			WithLineHandler(func(stream, line string) {
				if strings.HasPrefix(strings.TrimSpace(line), "{") {
					fmt.Fprintf(w, "data: %s\n\n", line)
				} else {
					data, _ := json.Marshal(map[string]string{"type": "log", "message": line})
					fmt.Fprintf(w, "data: %s\n\n", data)
				}
				flusher.Flush()
			}).
			Run()

		if err != nil {
			secretsLog.Error("nh os switch failed", "error", err)
			errData, _ := json.Marshal(map[string]string{"type": "error", "message": fmt.Sprintf("deploy failed: %s", err)})
			fmt.Fprintf(w, "data: %s\n\n", errData)
		} else {
			setDeployPending(false)
			secretsLog.Info("secrets deploy completed successfully")
			doneData, _ := json.Marshal(map[string]string{"type": "done", "message": "Deploy completed successfully"})
			fmt.Fprintf(w, "data: %s\n\n", doneData)
		}
		flusher.Flush()
	})
}

// writeSecretsRegistry writes the JSON registry to /etc/homelab-secrets-registry.json.
// Called from the NixOS activation script — NOT from this Go code.
// This function exists only to document the expected JSON shape.
func _registryShape() {
	_ = []SecretEntry{
		{
			Name:        "caddy/CLOUDFLARE_API_TOKEN",
			Description: "Cloudflare DNS-01 API token for Caddy wildcard cert",
			Services:    []string{"caddy.service"},
			RunPath:     filepath.Join("/run/secrets", "caddy", "CLOUDFLARE_API_TOKEN"),
		},
	}
}
