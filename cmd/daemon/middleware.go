// HTTP middleware for the daemon API: request-id propagation, structured
// logging, and request counters for /metrics.
//
// Auth posture: the daemon listens on a unix socket with mode 0660 and
// group homelab-daemon. Only members of that group (including caddy) can
// connect. The peerUIDGuard SO_PEERCRED check was removed in Phase 3 —
// socket group permissions are sufficient for a single-host LAN.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"sync"
	"syscall"
	"time"

	logging "github.com/ogglord/homelab-logging"
)

var middlewareLog = logging.Logger("middleware")

// genRequestID produces a 16-hex-char random id when X-Request-Id is absent.
func genRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// requestIDLoggerMiddleware injects a request id into the context, echoes
// it on the response, and emits one structured log line per request. The
// id is propagated from the inbound X-Request-Id header (so the dash→daemon
// hop preserves it) or generated when absent.
func requestIDLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = genRequestID()
		}
		w.Header().Set("X-Request-Id", reqID)
		ctx := context.WithValue(r.Context(), logging.CtxKeyReqID, reqID)
		sr := &statusRecorder{ResponseWriter: w, status: 200}
		start := time.Now()
		next.ServeHTTP(sr, r.WithContext(ctx))
		dur := time.Since(start)
		// _msg is the scannable summary; structured fields carry the detail.
		msg := fmt.Sprintf("%s %s %d %dms", r.Method, r.URL.Path, sr.status, dur.Milliseconds())
		// Log at the appropriate level based on HTTP status:
		// 5xx = ERROR, 4xx = WARN, everything else = INFO
		switch {
		case sr.status >= 500:
			middlewareLog.Error(msg,
				"method", r.Method,
				"path", r.URL.Path,
				"status", sr.status,
				"duration_ms", dur.Milliseconds(),
				"req_id", reqID,
			)
		case sr.status >= 400:
			middlewareLog.Warn(msg,
				"method", r.Method,
				"path", r.URL.Path,
				"status", sr.status,
				"duration_ms", dur.Milliseconds(),
				"req_id", reqID,
			)
		default:
			middlewareLog.Info(msg,
				"method", r.Method,
				"path", r.URL.Path,
				"status", sr.status,
				"duration_ms", dur.Milliseconds(),
				"req_id", reqID,
			)
		}
		recordRequest(r.Method, sr.status, dur)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// peerCredListener wraps a *net.UnixListener so every accepted connection
// stores its SO_PEERCRED credentials. The http.Server.ConnContext hook can
// then surface the uid into per-request context.
type peerCredListener struct {
	*net.UnixListener
	mu    sync.Mutex
	creds map[net.Conn]*syscall.Ucred
}

func wrapListener(ln net.Listener) (*peerCredListener, bool) {
	ul, ok := ln.(*net.UnixListener)
	if !ok {
		return nil, false
	}
	return &peerCredListener{UnixListener: ul, creds: make(map[net.Conn]*syscall.Ucred)}, true
}

func (l *peerCredListener) Accept() (net.Conn, error) {
	c, err := l.UnixListener.AcceptUnix()
	if err != nil {
		return nil, err
	}
	if raw, rerr := c.SyscallConn(); rerr == nil {
		_ = raw.Control(func(fd uintptr) {
			if cred, gerr := syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED); gerr == nil {
				l.mu.Lock()
				l.creds[c] = cred
				l.mu.Unlock()
			}
		})
	}
	return c, nil
}

func (l *peerCredListener) creditFor(c net.Conn) *syscall.Ucred {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.creds[c]
}

// connContext is installed on http.Server.ConnContext; it pulls the cached
// SO_PEERCRED creds off the listener and stores the uid in r.Context().
func (l *peerCredListener) connContext(ctx context.Context, c net.Conn) context.Context {
	if cred := l.creditFor(c); cred != nil {
		ctx = context.WithValue(ctx, logging.CtxKeyPeerUID, cred.Uid)
	}
	return ctx
}

// peerUIDGuard is a no-op since Phase 3.
// The dash Go binary was removed; Caddy connects via the unix socket
// and socket group permissions (0660, group homelab-daemon) are sufficient.
// Previously this guarded destructive endpoints via SO_PEERCRED.
func peerUIDGuard(next http.Handler) http.Handler {
	return next
}

