package vpn

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublishPortAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vpn", "forwarded-port")

	if err := PublishPort(path, 48261); err != nil {
		t.Fatalf("PublishPort: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(b) != "48261" {
		t.Fatalf("got %q, want %q", string(b), "48261")
	}

	// Overwrite with a new value.
	if err := PublishPort(path, 50000); err != nil {
		t.Fatalf("PublishPort 2: %v", err)
	}
	b, _ = os.ReadFile(path)
	if string(b) != "50000" {
		t.Fatalf("got %q after rewrite, want 50000", string(b))
	}

	// No leftover temp files in the dir.
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, found %d (temp leak?)", len(entries))
	}
}
