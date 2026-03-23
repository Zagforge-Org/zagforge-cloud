package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/LegationPro/zagforge/api/internal/service/aiprovider/sseutil"
)

const defaultEndpoint = "https://api.openai.com/v1/chat/completions"
const defaultModel = "gpt-4o"

// Provider streams from the OpenAI Chat Completions API.
type Provider struct {
	Model    string
	Endpoint string
}

// New returns an OpenAI provider with default settings.
func New() *Provider {
	return &Provider{Model: defaultModel, Endpoint: defaultEndpoint}
}

type request struct {
	Model    string    `json:"model"`
	Stream   bool      `json:"stream"`
	Messages []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func (p *Provider) Stream(ctx context.Context, w http.ResponseWriter, apiKey, prompt string) error {
	if err := sseutil.ValidateInput(apiKey, prompt); err != nil {
		return err
	}

	req, err := sseutil.NewJSONRequest(ctx, p.Endpoint, request{
		Model:    p.Model,
		Stream:   true,
		Messages: []message{{Role: "user", Content: prompt}},
	}, map[string]string{
		"Authorization": "Bearer " + apiKey,
	})
	if err != nil {
		return fmt.Errorf("openai: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		sseutil.WriteError(w, "openai request failed")
		return fmt.Errorf("openai: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		sseutil.WriteError(w, fmt.Sprintf("openai returned status %d", resp.StatusCode))
		return fmt.Errorf("openai: status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		data, ok := sseutil.ParseSSELine(scanner.Text())
		if !ok {
			continue
		}
		if data == "[DONE]" {
			break
		}
		var c chunk
		if err := json.Unmarshal([]byte(data), &c); err != nil {
			continue
		}
		if len(c.Choices) > 0 && c.Choices[0].Delta.Content != "" {
			sseutil.WriteChunk(w, c.Choices[0].Delta.Content)
		}
	}

	sseutil.WriteDone(w)
	return nil
}
