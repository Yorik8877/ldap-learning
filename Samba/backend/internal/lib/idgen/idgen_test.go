package idgen_test

import (
	"regexp"
	"testing"

	"samba-admin/internal/lib/idgen"
)

const generatedCount = 1000

// crypto/rand.Text: не меньше 128 бит в алфавите base32 (RFC 4648) — это 26 символов A–Z и 2–7.
// Такие символы можно класть в cookie без экранирования.
var cookieSafeID = regexp.MustCompile(`^[A-Z2-7]{26,}$`)

func TestNewIDIsLongCookieSafeText(t *testing.T) {
	id, err := idgen.New().NewID()
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}
	if !cookieSafeID.MatchString(id) {
		t.Fatalf("NewID() = %q, want at least 26 base32 characters", id)
	}
}

func TestNewIDDoesNotRepeat(t *testing.T) {
	generator := idgen.New()
	seen := make(map[string]bool, generatedCount)

	for range generatedCount {
		id, err := generator.NewID()
		if err != nil {
			t.Fatalf("NewID() error = %v", err)
		}
		if seen[id] {
			t.Fatalf("NewID() repeated %q", id)
		}
		seen[id] = true
	}
}
