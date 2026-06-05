// Package logging is the single source of slog loggers for homelab-daemon
// and homelab-dash. Every Go package in those binaries MUST obtain its
// logger via Logger(module) — direct slog.Default()/slog.Info() calls are
// forbidden and CI-linted.
//
// Field contract:
//   module: required, low-cardinality, one of the canonical names listed
//           in logging.md.
//   kind:   "event" by default; CmdLogger() returns "cmd" for shell
//           invocations emitted by cmdrunner.
package logging

import (
	"context"
	"log/slog"
	"os"
	"sync"
)

// canonicalModules is the allowed module-name set, mirrored from logging.md.
// Add new modules here and to logging.md in the same change.
var canonicalModules = map[string]struct{}{
	// daemon
	"bug": {}, "api": {}, "secrets": {}, "monitor": {}, "middleware": {},
	"storage": {}, "updates": {}, "cmdrunner": {}, "daemon_collector": {},
	// dash
	"dash_server": {}, "dash_daemon": {}, "dash_storage": {}, "dash_bug": {},
}

var (
	once    sync.Once
	rootLog *slog.Logger
)

// Init installs a JSON handler on slog.Default() once. Safe to call from
// multiple init() blocks. The daemon's main and dash's main should also
// call it explicitly before any goroutine logs.
func Init() {
	once.Do(func() {
		h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
		rootLog = slog.New(h)
		slog.SetDefault(rootLog)
	})
}

// Logger returns a slog.Logger pre-tagged with module=<m>, kind=event.
// Panics if m is not in canonicalModules — that's the discipline-enforcer.
func Logger(m string) *slog.Logger {
	if _, ok := canonicalModules[m]; !ok {
		panic("logging: module " + m + " not in canonicalModules; add it to pkg/logging/logging.go and logging.md")
	}
	Init()
	return slog.Default().With("module", m, "kind", "event")
}

// CmdLogger returns a logger tagged with module=cmdrunner, kind=cmd.
// Only cmdrunner.Run() should use this. Callers of cmdrunner keep using
// Logger("<their module>") for the surrounding event log.
func CmdLogger() *slog.Logger {
	Init()
	return slog.Default().With("module", "cmdrunner", "kind", "cmd")
}

// CtxKey is the exported context-key type. Defined here (and not in the
// daemon's middleware) so both pkg/logging.FromContext AND the daemon's
// middleware refer to the same key value — context.Value() lookups are
// keyed by interface identity, so a separate unexported type in
// middleware.go would not collide with this one and FromContext would
// silently see nil. ALL packages that read/write these keys must import
// this type.
type CtxKey int

const (
	CtxKeyReqID   CtxKey = iota // string: request id propagated by HTTP middleware
	CtxKeyPeerUID                // uint32: SO_PEERCRED uid (daemon middleware only)
)

// FromContext extracts a logger pre-tagged with the request id, if present.
// Used by API handlers so every line within a request shares a req_id.
// Falls back to the supplied base logger unchanged when no id is set.
func FromContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	out := base
	if v := ctx.Value(CtxKeyReqID); v != nil {
		if s, ok := v.(string); ok && s != "" {
			out = out.With("req_id", s)
		}
	}
	return out
}
