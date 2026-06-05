package api

// UpdateInfo is per-container update state.
type UpdateInfo struct {
	HasUpdate      bool   `json:"has_update"`
	CurrentVersion string `json:"current_version"`
	RemoteVersion  string `json:"remote_version"`
	LocalID        string `json:"local_id"`
	RemoteID       string `json:"remote_id"`
}

// MetadataEntry is per-container static metadata (image, description, link).
type MetadataEntry struct {
	Image       string `json:"image"`
	Description string `json:"description"`
	RevisionURL string `json:"revision_url"`
}

// UpdatesStatus is the body of GET /api/v1/updates.
type UpdatesStatus struct {
	Updates  map[string]UpdateInfo    `json:"updates"`
	Metadata map[string]MetadataEntry `json:"metadata"`
}
