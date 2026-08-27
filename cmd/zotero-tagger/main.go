package main

import (
	"fmt"
	"os"

	"github.com/andreassag/zotero-tagger/internal/config"
	"github.com/andreassag/zotero-tagger/internal/pipeline"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	opts    pipeline.Options
)

var rootCmd = &cobra.Command{
	Use:     "zotero-tagger",
	Version: version,
	Short:   "Automatically tag academic papers in Zotero libraries using LLM taxonomy extraction",
}

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Run the fetching, processing, tagging, and sync pipeline",
	RunE: func(cmd *cobra.Command, args []string) error {
		logLevel := zerolog.InfoLevel
		if opts.Verbose {
			logLevel = zerolog.DebugLevel
		}
		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
			Level(logLevel).
			With().
			Timestamp().
			Logger()

		cfg, err := config.LoadConfig(opts.ConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		runner, err := pipeline.NewRunner(cmd.Context(), cfg, logger)
		if err != nil {
			return fmt.Errorf("failed to initialize pipeline runner: %w", err)
		}
		return runner.Run(cmd.Context(), opts)
	},
}

func init() {
	tagCmd.Flags().StringVar(&opts.ConfigPath, "config", "config/config.toml", "Path to config file")
	tagCmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Preview tags without updating Zotero")
	tagCmd.Flags().IntVar(&opts.Limit, "limit", 0, "Max items to process (0 = all)")
	tagCmd.Flags().StringVar(&opts.CollectionKey, "collection", "", "Zotero collection key")
	tagCmd.Flags().StringVar(&opts.GroupID, "group", "", "Zotero group ID")
	tagCmd.Flags().StringVar(&opts.ItemKey, "item", "", "Single Zotero item key to process")
	tagCmd.Flags().BoolVar(&opts.Batch, "batch", false, "Use LLM batch API mode")
	tagCmd.Flags().IntVar(&opts.Concurrent, "concurrent", 1, "Number of concurrent workers")
	tagCmd.Flags().BoolVar(&opts.Reprocess, "reprocess", false, "Reprocess items even if sentinel tag exists")
	tagCmd.Flags().BoolVar(&opts.Verbose, "verbose", false, "Enable DEBUG logging")
	tagCmd.Flags().BoolVar(&opts.UseCache, "cache", false, "Cache LLM prompt responses on disk to save tokens during testing")
	tagCmd.Flags().BoolVar(&opts.SkipLLM, "skip-llm", false, "Skip LLM calls and Zotero tag updates to inspect text reduction only")

	rootCmd.AddCommand(tagCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
