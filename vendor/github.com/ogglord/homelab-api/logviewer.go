package api

// LogViewerConfig is the schema for the dashboard's log viewer page.
// Generated at runtime by the daemon from its managed services list
// so every service in services.yaml automatically gets a filter tab.
type LogViewerConfig struct {
	DefaultMode  string                  `json:"defaultMode"`
	DefaultWindow string                 `json:"defaultWindow"`
	MaxLines     int                     `json:"maxLines"`
	Services     []LogViewerServiceConfig `json:"services"`
}

type LogViewerServiceConfig struct {
	ID       string           `json:"id"`
	Label    string           `json:"label"`
	Selector string           `json:"selector"` // LogsQL selector, e.g. {unit="myservice.service"}
	Fields   []LogViewerField `json:"fields"`
	Format   string           `json:"format"`
}

type LogViewerField struct {
	Name   string   `json:"name"`
	Label  string   `json:"label"`
	Type   string   `json:"type"` // "enum" | "text" | "number"
	Values []string `json:"values,omitempty"`
}
