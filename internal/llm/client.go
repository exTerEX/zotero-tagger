package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/andreassag/zotero-tagger/internal/config"
	"github.com/rs/zerolog"
	"google.golang.org/genai"
)

type Client struct {
	client      *genai.Client
	model       string
	temperature float64
	rateLimiter *RateLimiter
	retryConfig config.RetryConfig
	logger      zerolog.Logger
	cache       *DiskCache
}

func NewClient(ctx context.Context, cfg config.LLMConfig, logger zerolog.Logger) (*Client, error) {
	clientConfig := &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	}

	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Google GenAI client: %w", err)
	}

	return &Client{
		client:      client,
		model:       cfg.ModelName,
		temperature: cfg.Temperature,
		rateLimiter: NewRateLimiter(cfg.RateLimits),
		retryConfig: cfg.Retries,
		logger:      logger,
		cache:       NewDiskCache(),
	}, nil
}

func (c *Client) CallSync(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Word-count proxy (1 word ≈ 1.3 tokens) — consistent with TruncateToTokenBudget
	estimatedTokens := int(float64(len(strings.Fields(systemPrompt))+len(strings.Fields(userPrompt))) * 1.3)
	if err := c.rateLimiter.Wait(ctx, estimatedTokens); err != nil {
		return "", fmt.Errorf("rate limiter wait failed: %w", err)
	}

	temp := float32(c.temperature)
	genConfig := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemPrompt},
			},
		},
		Temperature:      &temp,
		ResponseMIMEType: "application/json",
	}

	resp, err := c.client.Models.GenerateContent(ctx, c.model, genai.Text(userPrompt), genConfig)
	if err != nil {
		return "", fmt.Errorf("gemini generate content failed: %w", err)
	}

	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response choices returned from LLM")
	}

	result := resp.Text()
	if result == "" {
		var sb strings.Builder
		for _, part := range resp.Candidates[0].Content.Parts {
			if part.Text != "" {
				sb.WriteString(part.Text)
			}
		}
		result = sb.String()
	}

	if result == "" {
		return "", fmt.Errorf("empty text response returned from LLM")
	}

	return result, nil
}

func (c *Client) CallSyncWithCache(ctx context.Context, systemPrompt, userPrompt string, useCache bool) (string, error) {
	hash := HashPrompt(c.model, systemPrompt, userPrompt)
	if useCache && c.cache != nil {
		if cached, ok := c.cache.Get(hash); ok {
			c.logger.Info().Str("hash", hash[:8]).Msg("[CACHE HIT] Using cached LLM response")
			return cached, nil
		}
	}

	resp, err := c.CallSync(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	if useCache && c.cache != nil {
		_ = c.cache.Set(hash, resp)
	}

	return resp, nil
}
