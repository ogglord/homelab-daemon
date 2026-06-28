package vpn

import "testing"

func TestParseMappedPort(t *testing.T) {
	out := `initnatpmp() returned 0 (SUCCESS)
sendpublicaddressrequest returned 2 (SUCCESS)
Mapped public port 48261 protocol UDP to local port 0 lifetime 60
readnatpmpresponseorretry returned 0 (OK)`
	port, err := ParseMappedPort(out)
	if err != nil {
		t.Fatalf("ParseMappedPort: %v", err)
	}
	if port != 48261 {
		t.Fatalf("got %d, want 48261", port)
	}
}

func TestParseMappedPortMissing(t *testing.T) {
	if _, err := ParseMappedPort("no port here"); err == nil {
		t.Fatal("expected error for missing port")
	}
}
