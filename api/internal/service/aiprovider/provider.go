package aiprovider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/LegationPro/zagforge/api/internal/service/aiprovider/anthropic"
	"github.com/LegationPro/zagforge/api/internal/service/aiprovider/gemini"
	"github.com/LegationPro/zagforge/api/internal/service/aiprovider/openai"
	"github.com/LegationPro/zagforge/api/internal/service/aiprovider/xai"
)

// Provider streams an AI response to the client via SSE.
type Provider interface {
	Stream(ctx context.Context, w http.ResponseWriter, apiKey, prompt string) error
}

// New returns the Provider for the named provider string.
func New(name string) (Provider, error) {
	switch name {
	case "openai":
		return openai.New(), nil
	case "anthropic":
		return anthropic.New(), nil
	case "google":
		return gemini.New(), nil
	case "xai":
		return xai.New(), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}
