package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/andreassag/zotero-tagger/internal/config"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	rpm        *rate.Limiter
	tpm        *rate.Limiter
	mu         sync.Mutex
	dailyCount int
	dailyLimit int
	dailyReset time.Time
}

type PersistedState struct {
	DailyCount int       `json:"daily_count"`
	DailyReset time.Time `json:"daily_reset"`
}

func getStateFilePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ".zotero-tagger-state.json"
	}
	dir := filepath.Join(configDir, "zotero-tagger")
	_ = os.MkdirAll(dir, 0750)
	return filepath.Join(dir, "state.json")
}

func loadPersistedState() PersistedState {
	path := getStateFilePath()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return PersistedState{DailyCount: 0, DailyReset: time.Now().Add(24 * time.Hour)}
	}
	var state PersistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return PersistedState{DailyCount: 0, DailyReset: time.Now().Add(24 * time.Hour)}
	}
	return state
}

func savePersistedState(state PersistedState) {
	path := getStateFilePath()
	data, err := json.MarshalIndent(state, "", "  ")
	if err == nil {
		_ = os.WriteFile(filepath.Clean(path), data, 0600)
	}
}

func NewRateLimiter(cfg config.RateLimitConfig) *RateLimiter {
	var rpmLimiter *rate.Limiter
	if cfg.RequestsPerMinute > 0 {
		limit := rate.Every(time.Minute / time.Duration(cfg.RequestsPerMinute))
		rpmLimiter = rate.NewLimiter(limit, 1)
	} else {
		rpmLimiter = rate.NewLimiter(rate.Inf, 1)
	}

	var tpmLimiter *rate.Limiter
	if cfg.TokensPerMinute > 0 {
		limit := rate.Limit(float64(cfg.TokensPerMinute) / 60.0)
		tpmLimiter = rate.NewLimiter(limit, cfg.TokensPerMinute)
	} else {
		tpmLimiter = rate.NewLimiter(rate.Inf, 1000000000)
	}

	state := loadPersistedState()
	now := time.Now()
	if now.After(state.DailyReset) {
		state.DailyCount = 0
		state.DailyReset = now.Add(24 * time.Hour)
		savePersistedState(state)
	}

	return &RateLimiter{
		rpm:        rpmLimiter,
		tpm:        tpmLimiter,
		dailyLimit: cfg.RequestsPerDay,
		dailyCount: state.DailyCount,
		dailyReset: state.DailyReset,
	}
}

func (rl *RateLimiter) Wait(ctx context.Context, estimatedTokens int) error {
	rl.mu.Lock()
	now := time.Now()
	if now.After(rl.dailyReset) {
		rl.dailyCount = 0
		rl.dailyReset = now.Add(24 * time.Hour)
	}
	if rl.dailyLimit > 0 && rl.dailyCount >= rl.dailyLimit {
		rl.mu.Unlock()
		return fmt.Errorf("daily rate limit exceeded (%d/%d requests used today)", rl.dailyCount, rl.dailyLimit)
	}
	rl.dailyCount++
	savePersistedState(PersistedState{DailyCount: rl.dailyCount, DailyReset: rl.dailyReset})
	rl.mu.Unlock()

	if rl.rpm.Limit() != rate.Inf {
		if err := rl.rpm.Wait(ctx); err != nil {
			return err
		}
	}

	if estimatedTokens > 0 && rl.tpm.Limit() != rate.Inf {
		if err := rl.tpm.WaitN(ctx, estimatedTokens); err != nil {
			return err
		}
	}

	return nil
}
