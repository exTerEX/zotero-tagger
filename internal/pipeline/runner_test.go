package pipeline

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andreassag/zotero-tagger/internal/config"
	"github.com/andreassag/zotero-tagger/internal/display"
	"github.com/andreassag/zotero-tagger/internal/llm"
	"github.com/andreassag/zotero-tagger/internal/zotero"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

func newTestRunner(zoteroURL, geminiURL string, httpClient *http.Client) (*Runner, error) {
	cfg := &config.Config{
		LLM: config.LLMConfig{
			ModelName:   "gemini-3.5-flash-lite",
			Temperature: 0.0,
			RateLimits: config.RateLimitConfig{
				RequestsPerMinute: 0,
				TokensPerMinute:   0,
				RequestsPerDay:    0,
			},
		},
		Zotero: config.ZoteroConfig{
			UserID:      "12345",
			APIKey:      "test-key",
			LibraryType: "user",
			ItemTypes: config.ItemTypes{
				Allowed: []string{"journalArticle"},
			},
		},
		Tagging: config.TaggingConfig{
			SentinelTag: "_ai-tagged",
			ControlledTopics: config.ControlledTopics{
				Topics: []string{"biofilm", "pcr"},
			},
		},
	}

	zClient := zotero.NewClient(cfg.Zotero)
	zClient.SetBaseURLForTest(zoteroURL)

	genaiClient, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:     "test-api-key",
		Backend:    genai.BackendGeminiAPI,
		HTTPClient: httpClient,
		HTTPOptions: genai.HTTPOptions{
			BaseURL: geminiURL,
		},
	})
	if err != nil {
		return nil, err
	}

	llmClient := llm.NewClientForTest(genaiClient, cfg.LLM.ModelName, cfg.LLM.Temperature)

	return &Runner{
		cfg:          cfg,
		zoteroClient: zClient,
		llmClient:    llmClient,
		display:      display.NewOutput(),
		logger:       zerolog.Nop(),
	}, nil
}

func TestRunner_Run_NoItems(t *testing.T) {
	zoteroServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]zotero.Item{})
	}))
	defer zoteroServer.Close()

	runner, err := newTestRunner(zoteroServer.URL, "http://localhost", http.DefaultClient)
	require.NoError(t, err)

	err = runner.Run(context.Background(), Options{DryRun: true})
	assert.NoError(t, err)
}

func TestRunner_Run_SkipLLM(t *testing.T) {
	zoteroServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/12345/items/top" {
			items := []zotero.Item{
				{
					Key:     "ITEM1",
					Version: 1,
					Data: zotero.ItemData{
						ItemType:     "journalArticle",
						Title:        "Biofilm Study",
						AbstractNote: "This is an abstract about biofilm.",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(items)
			return
		}
		if r.URL.Path == "/users/12345/items/ITEM1/children" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]zotero.Item{})
			return
		}
	}))
	defer zoteroServer.Close()

	runner, err := newTestRunner(zoteroServer.URL, "http://localhost", http.DefaultClient)
	require.NoError(t, err)

	err = runner.Run(context.Background(), Options{
		SkipLLM: true,
	})
	assert.NoError(t, err)
}

func TestRunner_Run_SuccessDryRun(t *testing.T) {
	zoteroServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/12345/items/top" {
			items := []zotero.Item{
				{
					Key:     "ITEM1",
					Version: 1,
					Data: zotero.ItemData{
						ItemType:     "journalArticle",
						Title:        "Biofilm Study",
						AbstractNote: "This is an abstract about biofilm.",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(items)
			return
		}
		if r.URL.Path == "/users/12345/items/ITEM1/children" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]zotero.Item{})
			return
		}
	}))
	defer zoteroServer.Close()

	geminiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{
								"text": `{"org_tags":["org:pseudomonas-aeruginosa"],"group_tags":["group:gram-negative"],"topic_tags":["topic:biofilm"]}`,
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
	defer geminiServer.Close()

	runner, err := newTestRunner(zoteroServer.URL, geminiServer.URL, geminiServer.Client())
	require.NoError(t, err)

	err = runner.Run(context.Background(), Options{
		DryRun: true,
	})
	assert.NoError(t, err)
}

func TestRunner_Run_Cancellation(t *testing.T) {
	zoteroServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		items := []zotero.Item{
			{
				Key:     "ITEM1",
				Version: 1,
				Data: zotero.ItemData{
					ItemType:     "journalArticle",
					Title:        "Biofilm Study",
					AbstractNote: "Abstract text",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(items)
	}))
	defer zoteroServer.Close()

	runner, err := newTestRunner(zoteroServer.URL, "http://localhost", http.DefaultClient)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = runner.Run(ctx, Options{
		DryRun: true,
	})
	assert.NoError(t, err)
}
