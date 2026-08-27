package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/andreassag/zotero-tagger/internal/config"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"
)

func newTestClient(serverURL string, client *http.Client) (*Client, error) {
	genaiClient, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:     "test-api-key",
		Backend:    genai.BackendGeminiAPI,
		HTTPClient: client,
		HTTPOptions: genai.HTTPOptions{
			BaseURL: serverURL,
		},
	})
	if err != nil {
		return nil, err
	}

	rateCfg := config.RateLimitConfig{
		RequestsPerMinute: 0,
		TokensPerMinute:   0,
		RequestsPerDay:    0,
	}

	return &Client{
		client:      genaiClient,
		model:       "gemini-3.5-flash-lite",
		temperature: 0.0,
		rateLimiter: NewRateLimiter(rateCfg),
		retryConfig: config.RetryConfig{
			MaxRetries:  1,
			BackoffBase: 0.01,
		},
		logger: zerolog.Nop(),
		cache:  NewDiskCache(),
	}, nil
}

func TestCallSync_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)

		resp := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{
								"text": `{"org_tags":["org:escherichia-coli"],"group_tags":[],"topic_tags":["topic:pcr"]}`,
							},
						},
						"role": "model",
					},
					"finishReason": "STOP",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := newTestClient(server.URL, server.Client())
	assert.NoError(t, err)

	result, err := client.CallSync(context.Background(), "system prompt", "user prompt")
	assert.NoError(t, err)
	assert.Contains(t, result, "org:escherichia-coli")
}

func TestCallSync_EmptyCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"candidates": []interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := newTestClient(server.URL, server.Client())
	assert.NoError(t, err)

	_, err = client.CallSync(context.Background(), "system", "user")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no response choices")
}

func TestRateLimiter_DailyLimit(t *testing.T) {
	statePath := getStateFilePath()
	_ = os.Remove(statePath)
	defer func() { _ = os.Remove(statePath) }()

	cfg := config.RateLimitConfig{
		RequestsPerMinute: 0,
		TokensPerMinute:   0,
		RequestsPerDay:    2,
	}
	rl := NewRateLimiter(cfg)

	err1 := rl.Wait(context.Background(), 0)
	assert.NoError(t, err1)

	err2 := rl.Wait(context.Background(), 0)
	assert.NoError(t, err2)

	// Third request should be blocked (daily limit reached)
	err3 := rl.Wait(context.Background(), 0)
	assert.Error(t, err3)
}

func TestRateLimiter_Unlimited(t *testing.T) {
	cfg := config.RateLimitConfig{
		RequestsPerMinute: 0,
		TokensPerMinute:   0,
		RequestsPerDay:    0,
	}
	rl := NewRateLimiter(cfg)

	err := rl.Wait(context.Background(), 100)
	assert.NoError(t, err)
}
