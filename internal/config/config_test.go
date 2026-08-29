package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate_Success(t *testing.T) {
	cfg := Config{
		LLM: LLMConfig{
			ModelName: "gemini-3.5-flash-lite",
		},
		Zotero: ZoteroConfig{
			LibraryType: "user",
		},
		Tagging: TaggingConfig{
			ControlledTopics: ControlledTopics{
				Topics: []string{"biofilm", "pcr"},
			},
		},
	}

	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_MissingModelName(t *testing.T) {
	cfg := Config{
		LLM: LLMConfig{
			ModelName: "",
		},
		Zotero: ZoteroConfig{
			LibraryType: "user",
		},
		Tagging: TaggingConfig{
			ControlledTopics: ControlledTopics{
				Topics: []string{"biofilm"},
			},
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "llm.model_name is required")
}

func TestConfig_Validate_InvalidLibraryType(t *testing.T) {
	cfg := Config{
		LLM: LLMConfig{
			ModelName: "gemini-3.5-flash-lite",
		},
		Zotero: ZoteroConfig{
			LibraryType: "invalid_type",
		},
		Tagging: TaggingConfig{
			ControlledTopics: ControlledTopics{
				Topics: []string{"biofilm"},
			},
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "zotero.library_type must be either 'user' or 'group'")
}

func TestConfig_Validate_EmptyTopics(t *testing.T) {
	cfg := Config{
		LLM: LLMConfig{
			ModelName: "gemini-3.5-flash-lite",
		},
		Zotero: ZoteroConfig{
			LibraryType: "group",
		},
		Tagging: TaggingConfig{
			ControlledTopics: ControlledTopics{
				Topics: []string{},
			},
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must contain at least one controlled topic")
}

func TestLoadConfig_Success(t *testing.T) {
	tomlContent := `
[llm]
model_name = "gemini-3.5-flash-lite"
temperature = 0.2

[llm.rate_limits]
requests_per_minute = 100

[zotero]
library_type = "user"
user_id = "test_user"

[tagging.controlled_topics]
topics = ["biofilm", "oral microbiology"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(tomlContent), 0600)
	require.NoError(t, err)

	t.Setenv("ZOTERO_USER_ID", "")
	t.Setenv("ZOTERO_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, "gemini-3.5-flash-lite", cfg.LLM.ModelName)
	assert.Equal(t, 0.2, cfg.LLM.Temperature)
	assert.Equal(t, 100, cfg.LLM.RateLimits.RequestsPerMinute)
	assert.Equal(t, "user", cfg.Zotero.LibraryType)
	assert.Equal(t, "test_user", cfg.Zotero.UserID)
	assert.Equal(t, []string{"biofilm", "oral microbiology"}, cfg.Tagging.ControlledTopics.Topics)
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	tomlContent := `
[llm]
model_name = "gemini-3.5-flash-lite"

[zotero]
library_type = "user"

[tagging.controlled_topics]
topics = ["biofilm"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(tomlContent), 0600)
	require.NoError(t, err)

	t.Setenv("GEMINI_API_KEY", "env_gemini_key")
	t.Setenv("ZOTERO_USER_ID", "env_user_id")
	t.Setenv("ZOTERO_API_KEY", "env_zotero_key")

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, "env_gemini_key", cfg.LLM.APIKey)
	assert.Equal(t, "env_user_id", cfg.Zotero.UserID)
	assert.Equal(t, "env_zotero_key", cfg.Zotero.APIKey)
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("nonexistent_path_to_config.toml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}
