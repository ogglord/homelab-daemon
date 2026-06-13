package doctor_test

import (
	"testing"

	"github.com/ogglord/homelab-daemon/internal/doctor"
)

func TestRunSubset(t *testing.T) {
	// Run only the disk check — always available in test env
	report := doctor.Run([]string{"disk"})
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	if report.Results[0].Name != "disk" {
		t.Errorf("expected name 'disk', got %q", report.Results[0].Name)
	}
}

func TestRunAll(t *testing.T) {
	report := doctor.Run(nil) // nil = all checks
	if len(report.Results) == 0 {
		t.Fatal("expected at least one result")
	}
	total := report.Passed + report.Failed
	if total != len(report.Results) {
		t.Errorf("passed+failed=%d != len(results)=%d", total, len(report.Results))
	}
}

func TestRunUnknownCheck(t *testing.T) {
	report := doctor.Run([]string{"nonexistent-check"})
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	if report.Results[0].OK {
		t.Error("unknown check should not be OK")
	}
}
