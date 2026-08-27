package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	LLM        LLMConfig        `mapstructure:"llm"`
	Zotero     ZoteroConfig     `mapstructure:"zotero"`
	Processing ProcessingConfig `mapstructure:"processing"`
	Tagging    TaggingConfig    `mapstructure:"tagging"`
}

type RetryConfig struct {
	MaxRetries  int     `mapstructure:"max_retries"`
	BackoffBase float64 `mapstructure:"backoff_base"`
}

type RateLimitConfig struct {
	RequestsPerMinute int `mapstructure:"requests_per_minute"`
	TokensPerMinute   int `mapstructure:"tokens_per_minute"`
	RequestsPerDay    int `mapstructure:"requests_per_day"`
}

type LLMConfig struct {
	APIKey         string          `mapstructure:"api_key"`
	ModelName      string          `mapstructure:"model_name"`
	Temperature    float64         `mapstructure:"temperature"`
	MaxInputTokens int             `mapstructure:"max_input_tokens"`
	RateLimits     RateLimitConfig `mapstructure:"rate_limits"`
	Retries        RetryConfig     `mapstructure:"retries"`
}

type ZoteroConfig struct {
	UserID      string      `mapstructure:"user_id"`
	APIKey      string      `mapstructure:"api_key"`
	LibraryType string      `mapstructure:"library_type"`
	GroupID     string      `mapstructure:"group_id"`
	Retries     RetryConfig `mapstructure:"retries"`
	ItemTypes   ItemTypes   `mapstructure:"item_types"`
}

type ItemTypes struct {
	Allowed []string `mapstructure:"allowed"`
}

type ProcessingConfig struct {
	SectionPriority []string `mapstructure:"section_priority"`
	IntroParagraphs int      `mapstructure:"intro_paragraphs"`
}

type ControlledTopics struct {
	Topics []string `mapstructure:"topics"`
}

type TaggingConfig struct {
	SentinelTag      string           `mapstructure:"sentinel_tag"`
	ControlledTopics ControlledTopics `mapstructure:"controlled_topics"`
}

func LoadConfig(configPath string) (*Config, error) {
	// Try loading .env from current or parent directories
	envPaths := []string{".env", "../.env", "../../.env", "../../../.env"}
	for _, p := range envPaths {
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			break
		}
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("toml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if envKey := os.Getenv("GEMINI_API_KEY"); envKey != "" {
		cfg.LLM.APIKey = envKey
	}
	if envUserID := os.Getenv("ZOTERO_USER_ID"); envUserID != "" {
		cfg.Zotero.UserID = envUserID
	}
	if envZoteroKey := os.Getenv("ZOTERO_API_KEY"); envZoteroKey != "" {
		cfg.Zotero.APIKey = envZoteroKey
	}

	return &cfg, nil
}
