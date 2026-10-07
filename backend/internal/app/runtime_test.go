package app

import "testing"

func TestRandomIDGeneratorProducesDistinctURLSafeIDs(t *testing.T) {
	generator := randomIDGenerator{}

	first, firstErr := generator.NewID()
	second, secondErr := generator.NewID()

	if firstErr != nil || secondErr != nil {
		t.Fatalf("NewID() errors = %v, %v", firstErr, secondErr)
	}
	// 32 байта в base64url без выравнивания — 43 символа.
	if len(first) != 43 || first == second {
		t.Fatalf("NewID() = %q, %q", first, second)
	}
}
