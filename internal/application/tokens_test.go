package application

import (
	"encoding/hex"
	"testing"
)

func TestCryptoTokenGenerator_DefaultSize(t *testing.T) {
	g := NewCryptoTokenGenerator(0)
	tok, err := g.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tok) != defaultTokenBytes*2 {
		t.Errorf("default token: got %d hex chars, want %d", len(tok), defaultTokenBytes*2)
	}
	if _, err := hex.DecodeString(tok); err != nil {
		t.Errorf("token is not valid hex: %v", err)
	}
}

func TestCryptoTokenGenerator_CustomSize(t *testing.T) {
	g := NewCryptoTokenGenerator(8)
	tok, err := g.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tok) != 16 {
		t.Errorf("custom token: got %d hex chars, want 16", len(tok))
	}
}

func TestCryptoTokenGenerator_TokensAreUnique(t *testing.T) {
	g := NewCryptoTokenGenerator(0)
	seen := make(map[string]struct{}, 32)
	for range 32 {
		tok, err := g.Generate()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, dup := seen[tok]; dup {
			t.Fatalf("duplicate token produced: %s", tok)
		}
		seen[tok] = struct{}{}
	}
}
