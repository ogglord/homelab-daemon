package updates

import (
	"context"
	"encoding/json"
	"fmt"
	logging "github.com/ogglord/homelab-logging"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ogglord/homelab-daemon/internal/cmdrunner"
)

var log = logging.Logger("updates")

type UpdateInfo struct {
	HasUpdate      bool   `json:"has_update"`
	CurrentVersion string `json:"current_version"`
	RemoteVersion  string `json:"remote_version"`
	LocalID        string `json:"local_id"`
	RemoteID       string `json:"remote_id"`
}

type MetadataEntry struct {
	Image       string `json:"image"`
	Description string `json:"description"`
	RevisionURL string `json:"revision_url"`
}

type Module struct {
	updatesFile  string
	metadataFile string
	updates      map[string]UpdateInfo
	metadata     map[string]MetadataEntry
	mu           sync.RWMutex
	checkTrigger chan struct{}
}

func New(stateDir string) *Module {
	if stateDir == "" {
		stateDir = "/var/lib/homelab-daemon"
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		log.Error("updates: failed to create state directory", "dir", stateDir, "error", err)
	}

	m := &Module{
		updatesFile:  stateDir + "/podman-updates.json",
		metadataFile: stateDir + "/podman-metadata.json",
		updates:      make(map[string]UpdateInfo),
		metadata:     make(map[string]MetadataEntry),
		checkTrigger: make(chan struct{}, 1),
	}

	m.loadState()
	return m
}

func (m *Module) loadState() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if data, err := os.ReadFile(m.updatesFile); err == nil {
		if err := json.Unmarshal(data, &m.updates); err != nil {
			log.Error("updates: failed to unmarshal updates state", "file", m.updatesFile, "error", err)
		}
	} else if !os.IsNotExist(err) {
		log.Error("updates: failed to read updates state", "file", m.updatesFile, "error", err)
	}
	if data, err := os.ReadFile(m.metadataFile); err == nil {
		if err := json.Unmarshal(data, &m.metadata); err != nil {
			log.Error("updates: failed to unmarshal metadata state", "file", m.metadataFile, "error", err)
		}
	} else if !os.IsNotExist(err) {
		log.Error("updates: failed to read metadata state", "file", m.metadataFile, "error", err)
	}
	log.Info("Loaded state from persistent files", "updates", len(m.updates), "metadata", len(m.metadata))
}

func (m *Module) saveState() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if data, err := json.MarshalIndent(m.updates, "", "  "); err == nil {
		if err := os.WriteFile(m.updatesFile, data, 0o600); err != nil {
			log.Error("updates: failed to write updates state", "file", m.updatesFile, "error", err)
		}
	} else {
		log.Error("updates: failed to marshal updates state", "error", err)
	}
	if data, err := json.MarshalIndent(m.metadata, "", "  "); err == nil {
		if err := os.WriteFile(m.metadataFile, data, 0o600); err != nil {
			log.Error("updates: failed to write metadata state", "file", m.metadataFile, "error", err)
		}
	} else {
		log.Error("updates: failed to marshal metadata state", "error", err)
	}
}

func (m *Module) GetUpdates() map[string]UpdateInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]UpdateInfo)
	for k, v := range m.updates {
		out[k] = v
	}
	return out
}

func (m *Module) GetMetadata() map[string]MetadataEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]MetadataEntry)
	for k, v := range m.metadata {
		out[k] = v
	}
	return out
}

func (m *Module) TriggerCheck() {
	select {
	case m.checkTrigger <- struct{}{}:
	default:
		// Check is already pending
	}
}

func (m *Module) Start(ctx context.Context) {
	log.Info("Starting native container updates checker worker")

	// Initial check on boot (asynchronously so it doesn't block startup)
	m.TriggerCheck()

	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("Stopping container updates checker worker")
			return
		case <-ticker.C:
			m.TriggerCheck()
		case <-m.checkTrigger:
			log.Info("Running container metadata and update checks...")
			m.runChecks(ctx)
			log.Info("Container checks completed.")
		}
	}
}

type podmanImageInspect struct {
	Digest      string            `json:"Digest"`
	RepoDigests []string          `json:"RepoDigests"`
	Labels      map[string]string `json:"Labels"`
	Config      struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

type skopeoRawManifest struct {
	MediaType string `json:"mediaType"`
	Manifests []struct {
		Digest   string `json:"digest"`
		Platform struct {
			Architecture string `json:"architecture"`
			OS           string `json:"os"`
		} `json:"platform"`
	} `json:"manifests"`
	Config struct {
		Digest string `json:"digest"`
	} `json:"config"`
	Digest string `json:"Digest"`
}

type skopeoInspect struct {
	Labels map[string]string `json:"Labels"`
}

func (m *Module) runChecks(ctx context.Context) {
	// Query podman ps -a with custom format fields
	cmdRes, err := cmdrunner.New("updates", "podman", "ps", "-a", "--format", "{{.Names}}\t{{.Image}}\t{{index .Labels \"homepage.description\"}}\t{{index .Labels \"org.opencontainers.image.description\"}}\t{{index .Labels \"description\"}}\t{{index .Labels \"org.opencontainers.image.revision\"}}\t{{index .Labels \"org.opencontainers.image.source\"}}").
		WithContext(ctx).
		Run()
	if err != nil {
		log.Error("podman ps command failed", "error", err)
		return
	}

	lines := strings.Split(strings.TrimSpace(cmdRes.Stdout), "\n")
	newUpdates := make(map[string]UpdateInfo)
	newMetadata := make(map[string]MetadataEntry)

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		image := strings.TrimSpace(parts[1])
		if name == "" || image == "" {
			continue
		}

		// Pick the best description
		desc := ""
		for _, idx := range []int{2, 3, 4} {
			if idx < len(parts) {
				val := strings.TrimSpace(parts[idx])
				if val != "" && val != "<nil>" {
					desc = val
					break
				}
			}
		}

		// Revision URL
		revUrl := ""
		var rev, sourceUrl string
		if len(parts) > 5 {
			rev = strings.TrimSpace(parts[5])
		}
		if len(parts) > 6 {
			sourceUrl = strings.TrimSpace(parts[6])
		}

		if rev != "" && rev != "<nil>" && sourceUrl != "" && sourceUrl != "<nil>" {
			sourceUrl = strings.TrimSuffix(sourceUrl, ".git")
			if strings.Contains(sourceUrl, "github.com") || strings.Contains(sourceUrl, "gitlab.com") {
				revUrl = fmt.Sprintf("%s/commit/%s", sourceUrl, rev)
			} else {
				revUrl = sourceUrl
			}
		}

		newMetadata[name] = MetadataEntry{
			Image:       image,
			Description: desc,
			RevisionURL: revUrl,
		}

		// Now query local and remote digests
		log.Debug("Checking image update state", "name", name, "image", image)
		upInfo := m.checkSingleImage(ctx, image)
		newUpdates[name] = upInfo
	}

	// Update cached structures
	m.mu.Lock()
	m.updates = newUpdates
	m.metadata = newMetadata
	m.mu.Unlock()

	m.saveState()
}

func (m *Module) checkSingleImage(ctx context.Context, image string) UpdateInfo {
	fallback := UpdateInfo{HasUpdate: false, LocalID: "unknown", RemoteID: "unknown"}

	// Local inspect
	inspectRes, err := cmdrunner.New("updates", "podman", "image", "inspect", image).
		WithContext(ctx).
		Run()
	if err != nil {
		return fallback
	}

	var inspectList []podmanImageInspect
	if err := json.Unmarshal([]byte(inspectRes.Stdout), &inspectList); err != nil || len(inspectList) == 0 {
		return fallback
	}
	local := inspectList[0]

	localDigest := local.Digest
	localRepoDigests := strings.Join(local.RepoDigests, " ")

	localVersion := local.Config.Labels["org.opencontainers.image.version"]
	if localVersion == "" {
		localVersion = local.Labels["version"]
	}

	// Query remote raw manifest via Skopeo
	skopeoRawRes, err := cmdrunner.New("updates", "skopeo", "inspect", "--raw", "docker://"+image).
		WithContext(ctx).
		Run()
	if err != nil {
		log.Warn("skopeo raw inspect failed", "image", image, "error", err)
		return fallback
	}

	var rawManifest skopeoRawManifest
	if err := json.Unmarshal([]byte(skopeoRawRes.Stdout), &rawManifest); err != nil {
		return fallback
	}

	remoteDigest := ""
	if strings.Contains(rawManifest.MediaType, "manifest.list") {
		for _, m := range rawManifest.Manifests {
			if m.Platform.Architecture == "amd64" && m.Platform.OS == "linux" {
				remoteDigest = m.Digest
				break
			}
		}
		if remoteDigest == "" && len(rawManifest.Manifests) > 0 {
			remoteDigest = rawManifest.Manifests[0].Digest
		}
	} else {
		remoteDigest = rawManifest.Config.Digest
		if remoteDigest == "" {
			remoteDigest = rawManifest.Digest
		}
	}

	if remoteDigest == "" {
		return fallback
	}

	remoteID := remoteDigest
	if idx := strings.Index(remoteID, ":"); idx != -1 {
		remoteID = remoteID[idx+1:]
	}
	if len(remoteID) > 12 {
		remoteID = remoteID[:12]
	}

	// Remote Version labels
	skopeoRes, err := cmdrunner.New("updates", "skopeo", "inspect", "docker://"+image).
		WithContext(ctx).
		Run()
	var remoteVersion string
	if err == nil {
		var normInspect skopeoInspect
		if err := json.Unmarshal([]byte(skopeoRes.Stdout), &normInspect); err == nil {
			remoteVersion = normInspect.Labels["org.opencontainers.image.version"]
			if remoteVersion == "" {
				remoteVersion = normInspect.Labels["version"]
			}
		}
	}

	hasUpdate := false
	localID := "unknown"

	if strings.Contains(localRepoDigests, remoteDigest) {
		localID = remoteID
	} else {
		hasUpdate = true
		localID = localDigest
		if idx := strings.Index(localID, ":"); idx != -1 {
			localID = localID[idx+1:]
		}
		if len(localID) > 12 {
			localID = localID[:12]
		}
	}

	return UpdateInfo{
		HasUpdate:      hasUpdate,
		CurrentVersion: localVersion,
		RemoteVersion:  remoteVersion,
		LocalID:        localID,
		RemoteID:       remoteID,
	}
}
