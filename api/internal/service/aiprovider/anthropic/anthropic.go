package anthropic

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/LegationPro/zagforge/api/internal/service/aiprovider/sseutil"
)

const defaultEndpoint = "https://api.anthropic.com/v1/messages"
const defaultModel = "claude-sonnet-4-20250514"

// Provider streams from the Anthropic Messages API.
type Provider struct {
	Model    string
	Endpoint string
}

// New returns an Anthropic provider with default settings.
func New() *Provider {
	return &Provider{Model: defaultModel, Endpoint: defaultEndpoint}
}

type request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Stream    bool      `json:"stream"`
	Messages  []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type delta struct {
	Type  string `json:"type"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
}

func (p *Provider) Stream(ctx context.Context, w http.ResponseWriter, apiKey, prompt string) error {
	if err := sseutil.ValidateInput(apiKey, prompt); err != nil {
		return err
	}

	req, err := sseutil.NewJSONRequest(ctx, p.Endpoint, request{
		Model:     p.Model,
		MaxTokens: 4096,
		Stream:    true,
		Messages:  []message{{Role: "user", Content: prompt}},
	}, map[string]string{
		"x-api-key":         apiKey,
		"anthropic-version": "2023-06-01",
	})
	if err != nil {
		return fmt.Errorf("anthropic: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		sseutil.WriteError(w, "anthropic request failed")
		return fmt.Errorf("anthropic: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		sseutil.WriteError(w, fmt.Sprintf("anthropic returned status %d", resp.StatusCode))
		return fmt.Errorf("anthropic: status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	var lastEvent string
	for scanner.Scan() {
		line := scanner.Text()

		if event, ok := strings.CutPrefix(line, "event: "); ok {
			lastEvent = event
			continue
		}

		data, ok := sseutil.ParseSSELine(line)
		if !ok {
			continue
		}

		if lastEvent == "message_stop" {
			break
		}
		if lastEvent != "content_block_delta" {
			continue
		}

		var d delta
		if err := json.Unmarshal([]byte(data), &d); err != nil {
			continue
		}
		if d.Delta.Text != "" {
			sseutil.WriteChunk(w, d.Delta.Text)
		}
	}

	sseutil.WriteDone(w)
	return nil
}
