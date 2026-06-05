package api

// SecretEntry mirrors the registry JSON written at activation by
// modules/secrets.nix to /etc/homelab-secrets-registry.json.
type SecretEntry struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Services    []string `json:"services"`
	RunPath     string   `json:"runPath"` // e.g. /run/secrets/caddy/CLOUDFLARE_API_TOKEN
}

// SecretStatus is the response shape for GET /api/v1/secrets[/:name].
// Embeds SecretEntry so all metadata fields are exposed.
type SecretStatus struct {
	SecretEntry
	Present    bool   `json:"present"`
	ModifiedAt string `json:"modified_at,omitempty"` // RFC3339; empty if unknown
	Preview    string `json:"preview,omitempty"`     // first 3 chars + "•••"
}

// SecretsListResponse is the body of GET /api/v1/secrets.
type SecretsListResponse struct {
	Secrets       []SecretStatus `json:"secrets"`
	DeployPending bool           `json:"deploy_pending"`
}

// SecretSetRequest is the body of PUT /api/v1/secrets/{name}.
type SecretSetRequest struct {
	Value string `json:"value"`
}

// SecretMutationResponse is returned by PUT/DELETE /api/v1/secrets/{name}.
type SecretMutationResponse struct {
	Success       bool `json:"success"`
	DeployPending bool `json:"deploy_pending"`
}

// DeployEvent is one frame of the SSE stream from
// POST /api/v1/secrets/deploy. Frames are JSON-encoded after `data: `.
type DeployEvent struct {
	Type    string `json:"type"` // "log" | "error" | "done"
	Message string `json:"message"`
}
