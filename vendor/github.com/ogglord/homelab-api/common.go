package api

// SuccessResponse is the canonical envelope for mutating endpoints
// that don't return a typed body. `Success` is always set; `Error` is
// populated on best-effort failures the handler swallowed (rare —
// most failures are returned via non-2xx + http.Error instead).
type SuccessResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// BulkResult is returned by /api/start-all and /api/stop-all.
type BulkResult struct {
	Success bool `json:"success"`
	Total   int  `json:"total"`
	Failed  int  `json:"failed"`
}

// ErrorResponse is the daemon's JSON error body for non-2xx responses.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}
