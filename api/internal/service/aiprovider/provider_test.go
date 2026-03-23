package aiprovider_test

import (
	"testing"

	"github.com/LegationPro/zagforge/api/internal/service/aiprovider"
)

func TestNew_AllProviders(t *testing.T) {
	providers := []string{"openai", "anthropic", "google", "xai"}
	for _, name := range providers {
		t.Run(name, func(t *testing.T) {
			p, err := aiprovider.New(name)
			if err != nil {
				t.Fatalf("New(%q): %v", name, err)
			}
			if p == nil {
				t.Fatalf("New(%q) returned nil", name)
			}
		})
	}
}

func TestNew_Unknown(t *testing.T) {
	_, err := aiprovider.New("unknown")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
