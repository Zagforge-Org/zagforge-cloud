package gemini

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/LegationPro/zagforge/api/internal/service/aiprovider/sseutil"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
const defaultModel = "gemini-2.0-flash"

// Provider streams from the Gemini API.
type Provider struct {
	Model   string
	BaseURL string
}

// New returns a Gemini provider with default settings.
func New() *Provider {
	return &Provider{Model: defaultModel, BaseURL: defaultBaseURL}
}

type request struct {
	Contents []content `json:"contents"`
}

type content struct {
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type chunk struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (p *Provider) Stream(ctx context.Context, w http.ResponseWriter, apiKey, prompt string) error {
	if err := sseutil.ValidateInput(apiKey, prompt); err != nil {
		return err
	}

	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s", p.BaseURL, p.Model, apiKey)

	req, err := sseutil.NewJSONRequest(ctx, url, request{
		Contents: []content{
			{Parts: []part{{Text: prompt}}},
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("gemini: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		sseutil.WriteError(w, "gemini request failed")
		return fmt.Errorf("gemini: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		sseutil.WriteError(w, fmt.Sprintf("gemini returned status %d", resp.StatusCode))
		return fmt.Errorf("gemini: status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		data, ok := sseutil.ParseSSELine(scanner.Text())
		if !ok {
			continue
		}
		var c chunk
		if err := json.Unmarshal([]byte(data), &c); err != nil {
			continue
		}
		if len(c.Candidates) > 0 && len(c.Candidates[0].Content.Parts) > 0 {
			if text := c.Candidates[0].Content.Parts[0].Text; text != "" {
				sseutil.WriteChunk(w, text)
			}
		}
	}

	sseutil.WriteDone(w)
	return nil
}
