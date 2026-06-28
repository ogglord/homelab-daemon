package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVPNStatusJSONTags(t *testing.T) {
	b, _ := json.Marshal(VPNStatus{Connected: true, ForwardedPort: 42})
	s := string(b)
	for _, want := range []string{`"Connected":true`, `"ForwardedPort":42`, `"Provider":""`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
}
