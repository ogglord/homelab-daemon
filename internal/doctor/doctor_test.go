package doctor_test

import (
	"strings"
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

func TestFormatReport(t *testing.T) {
	report := doctor.Report{
		Passed: 1,
		Failed: 1,
		Results: []doctor.Result{
			{Name: "socket", OK: true, Detail: "/run/homelab-daemon/daemon.sock"},
			{Name: "daemon", OK: false, Detail: "inactive", Fix: "sudo systemctl start homelab-daemon.service"},
		},
	}
	body := doctor.FormatReportText(report)
	if !strings.Contains(body, "[✔] socket") {
		t.Errorf("expected passing check in output, got:\n%s", body)
	}
	if !strings.Contains(body, "[✗] daemon") {
		t.Errorf("expected failing check in output, got:\n%s", body)
	}
	if !strings.Contains(body, "Fix:") {
		t.Errorf("expected Fix hint in output, got:\n%s", body)
	}
}
