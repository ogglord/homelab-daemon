// Package api is the shared wire contract between homelab-daemon and
// homelab-dash. It deliberately depends only on the standard library so
// that both consumers can import it cheaply and so that types can be
// machine-translated to TypeScript via tygo (see tygo.yaml).
//
// The contract is intentionally minimal: just JSON-tagged struct
// definitions plus a handful of constants. Behaviour (HTTP handlers,
// the typed client) lives in the consumers.
package api

// Version is the wire-contract version. Bump on breaking JSON changes.
const Version = "v1"

// APIPrefix is the route prefix for versioned endpoints.
// During the transition both /api/... and /api/v1/... are served.
const APIPrefix = "/api/" + Version

// SocketPath is the canonical unix-socket path the daemon listens on.
// Centralised here to avoid divergence across modules + Nix.
const SocketPath = "/run/homelab-daemon/daemon.sock"

// VersionResponse is the body of GET /api/version.
type VersionResponse struct {
	Version    string `json:"version"`
	APIVersion string `json:"apiVersion"`
	Prefix     string `json:"prefix"`
}
