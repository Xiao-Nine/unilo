package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"unilo/internal/config"
	"unilo/pkg/apperror"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type StreamDelta func(delta string) error

type CompleteResult struct {
	Content string
	Model   string
	Usage   map[string]any
}

type OpenAIClient struct {
	baseURL string
	apiKey  string
	model   string
	timeout time.Duration
	client  *http.Client
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type chatCompletionStreamChunk struct {
	Choices []struct {
		Delta ChatMessage `json:"delta"`
	} `json:"choices"`
	Usage map[string]any `json:"usage,omitempty"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Usage map[string]any `json:"usage,omitempty"`
}

func NewOpenAIClient(cfg config.AgentConfig) *OpenAIClient {
	return &OpenAIClient{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  strings.TrimSpace(cfg.APIKey),
		model:   strings.TrimSpace(cfg.Model),
		timeout: cfg.Timeout,
		client:  &http.Client{},
	}
}

func (c *OpenAIClient) Complete(ctx context.Context, messages []ChatMessage) (CompleteResult, error) {
	if c.baseURL == "" || c.apiKey == "" || c.model == "" {
		return CompleteResult{}, apperror.New(500, "agent provider is not configured")
	}
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	payload, err := json.Marshal(chatCompletionRequest{Model: c.model, Messages: messages, Temperature: 0.2})
	if err != nil {
		return CompleteResult{}, apperror.Internal(err)
	}
	resp, err := c.doCompletionRequest(ctx, payload)
	if err != nil {
		return CompleteResult{}, err
	}
	defer resp.Body.Close()
	var result chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return CompleteResult{}, apperror.Internal(err)
	}
	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return CompleteResult{}, apperror.Internal(fmt.Errorf("agent provider returned empty completion"))
	}
	return CompleteResult{Content: strings.TrimSpace(result.Choices[0].Message.Content), Model: c.model, Usage: result.Usage}, nil
}

func (c *OpenAIClient) StreamComplete(ctx context.Context, messages []ChatMessage, onDelta StreamDelta) (CompleteResult, error) {
	if c.baseURL == "" || c.apiKey == "" || c.model == "" {
		return CompleteResult{}, apperror.New(500, "agent provider is not configured")
	}
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	payload, err := json.Marshal(chatCompletionRequest{Model: c.model, Messages: messages, Temperature: 0.2, Stream: true})
	if err != nil {
		return CompleteResult{}, apperror.Internal(err)
	}
	resp, err := c.doCompletionRequest(ctx, payload)
	if err != nil {
		return CompleteResult{}, err
	}
	defer resp.Body.Close()

	var content strings.Builder
	usage := map[string]any{}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk chatCompletionStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return CompleteResult{}, apperror.Internal(err)
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		content.WriteString(delta)
		if onDelta != nil {
			if err := onDelta(delta); err != nil {
				return CompleteResult{}, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return CompleteResult{}, apperror.Internal(err)
	}
	final := strings.TrimSpace(content.String())
	if final == "" {
		return CompleteResult{}, apperror.Internal(fmt.Errorf("agent provider returned empty completion"))
	}
	return CompleteResult{Content: final, Model: c.model, Usage: usage}, nil
}

func (c *OpenAIClient) doCompletionRequest(ctx context.Context, payload []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, apperror.Internal(fmt.Errorf("agent provider returned status %d", resp.StatusCode))
	}
	return resp, nil
}
