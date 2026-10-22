package httpapi

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestNewPartnerSecret(t *testing.T) {
	first, err := newPartnerSecret()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newPartnerSecret()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("generated partner secrets must be unique")
	}
	if !strings.HasPrefix(first, "rdp_") {
		t.Fatalf("secret %q does not have the expected prefix", first)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(first, "rdp_"))
	if err != nil {
		t.Fatalf("decode generated secret: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("secret contains %d random bytes, want 32", len(decoded))
	}
}
