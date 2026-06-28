package api

// QbitTorrent is a single torrent surfaced on the dashboard Overview,
// aggregated across all qui-managed qBittorrent instances. Speeds are in
// bytes/sec, Size in bytes, Progress is 0..1, Eta in seconds. Instance is
// the qui instance name the torrent belongs to.
type QbitTorrent struct {
	Hash     string  `json:"hash"`
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Progress float64 `json:"progress"`
	Dlspeed  int64   `json:"dlspeed"`
	Upspeed  int64   `json:"upspeed"`
	Eta      int64   `json:"eta"`
	State    string  `json:"state"`
	Instance string  `json:"instance"`
}

// QbitStatus is the qBittorrent block of the dashboard Overview. Enabled is
// false when the qui integration is disabled; Error carries a qui fetch
// failure so the widget can show it without blocking the rest of the
// overview. Torrents is sorted by download speed, descending (active first).
type QbitStatus struct {
	Enabled  bool          `json:"enabled"`
	Error    string        `json:"error,omitempty"`
	Torrents []QbitTorrent `json:"torrents"`
}
