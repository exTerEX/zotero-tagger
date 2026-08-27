package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/andreassag/zotero-tagger/internal/config"
	"github.com/andreassag/zotero-tagger/internal/display"
	"github.com/andreassag/zotero-tagger/internal/llm"
	"github.com/andreassag/zotero-tagger/internal/processing"
	"github.com/andreassag/zotero-tagger/internal/tagging"
	"github.com/andreassag/zotero-tagger/internal/zotero"
	"github.com/rs/zerolog"
)

type Options struct {
	ConfigPath    string
	DryRun        bool
	Limit         int
	CollectionKey string
	GroupID       string
	ItemKey       string
	Batch         bool
	Concurrent    int
	Reprocess     bool
	Verbose       bool
	UseCache      bool
	SkipLLM       bool
}

type Runner struct {
	cfg          *config.Config
	zoteroClient *zotero.Client
	llmClient    *llm.Client
	display      *display.Output
	logger       zerolog.Logger
}

func NewRunner(ctx context.Context, cfg *config.Config, logger zerolog.Logger) (*Runner, error) {
	llmClient, err := llm.NewClient(ctx, cfg.LLM, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	return &Runner{
		cfg:          cfg,
		zoteroClient: zotero.NewClient(cfg.Zotero),
		llmClient:    llmClient,
		display:      display.NewOutput(),
		logger:       logger,
	}, nil
}

func (r *Runner) Run(ctx context.Context, opts Options) error {
	fetchOpts := zotero.FetchOptions{
		CollectionKey: opts.CollectionKey,
		GroupID:       opts.GroupID,
		ItemKey:       opts.ItemKey,
		Limit:         opts.Limit,
		ExcludeTag:    r.cfg.Tagging.SentinelTag,
		ItemTypes:     r.cfg.Zotero.ItemTypes.Allowed,
		Reprocess:     opts.Reprocess,
	}

	items, err := r.zoteroClient.FetchItems(ctx, fetchOpts)
	if err != nil {
		return fmt.Errorf("failed to fetch items from Zotero: %w", err)
	}

	if len(items) == 0 {
		r.logger.Info().Msg("No items found to process.")
		return nil
	}

	r.logger.Info().Int("count", len(items)).Msg("Starting item tagging pipeline...")

	total := len(items)
	var processed, skipped, errCount int
	var mu sync.Mutex

	if opts.Concurrent > 1 {
		sem := make(chan struct{}, opts.Concurrent)
		var wg sync.WaitGroup

		for _, item := range items {
			wg.Add(1)
			sem <- struct{}{}

			go func(it zotero.Item) {
				defer wg.Done()
				defer func() { <-sem }()

				err := r.processSingleItem(ctx, it, opts)
				mu.Lock()
				if err != nil {
					r.logger.Error().Err(err).Str("itemKey", it.Key).Msg("Failed to process item")
					errCount++
				} else {
					processed++
				}
				mu.Unlock()
			}(item)
		}

		wg.Wait()
	} else {
		for _, item := range items {
			err := r.processSingleItem(ctx, item, opts)
			if err != nil {
				r.logger.Error().Err(err).Str("itemKey", item.Key).Msg("Failed to process item")
				errCount++
			} else {
				processed++
			}
		}
	}

	r.display.RenderSummary(total, processed, skipped, errCount)
	return nil
}

func (r *Runner) processSingleItem(ctx context.Context, item zotero.Item, opts Options) error {
	r.display.RenderItemHeader(item.Data.Title, item.Key)

	var textToProcess string
	pdfPath, err := r.zoteroClient.DownloadPDF(ctx, item.Key, opts.GroupID)
	if err == nil && pdfPath != "" {
		defer func() { _ = os.Remove(pdfPath) }()
		extractedText, pdfErr := processing.ExtractTextFromPDF(pdfPath)
		if pdfErr == nil && strings.TrimSpace(extractedText) != "" {
			textToProcess = extractedText
		} else {
			r.logger.Warn().Err(pdfErr).Str("itemKey", item.Key).Msg("PDF text extraction failed; falling back to abstract")
			textToProcess = item.Data.AbstractNote
		}
	} else {
		r.logger.Info().Str("itemKey", item.Key).Msg("No PDF attachment found; using abstract note")
		textToProcess = item.Data.AbstractNote
	}

	if strings.TrimSpace(textToProcess) == "" {
		return fmt.Errorf("no text or abstract available for item %s", item.Key)
	}

	processed := processing.ProcessText(textToProcess, r.cfg.Processing, r.cfg.LLM.MaxInputTokens)
	r.display.RenderTokenSavings(processed.OriginalWordCount, processed.ProcessedWordCount)

	if opts.SkipLLM {
		r.logger.Info().Str("itemKey", item.Key).Msg("[SKIP-LLM] Skipping LLM call and Zotero update")
		return nil
	}

	existingTags := make([]string, len(item.Data.Tags))
	for i, t := range item.Data.Tags {
		existingTags[i] = t.Tag
	}

	sysPrompt := tagging.BuildSystemPrompt(r.cfg.Tagging.ControlledTopics.Topics)
	usrPrompt := tagging.BuildUserPrompt(item.Data.Title, processed.Text, processed.CandidateSpecies, existingTags)

	llmRawResp, err := r.llmClient.CallSyncWithCache(ctx, sysPrompt, usrPrompt, opts.UseCache)
	if err != nil {
		return fmt.Errorf("LLM API call failed: %w", err)
	}

	tagResult, err := tagging.ParseResponse(llmRawResp)
	if err != nil {
		return fmt.Errorf("failed to parse LLM output: %w", err)
	}

	tagResult = tagging.FilterControlledTopics(tagResult, r.cfg.Tagging.ControlledTopics.Topics)
	formattedTags := tagging.BuildTagList(tagResult, r.cfg.Tagging.SentinelTag)

	r.display.RenderTags(formattedTags)

	if opts.DryRun {
		r.logger.Info().Str("itemKey", item.Key).Msg("[DRY-RUN] Would update tags in Zotero")
		return nil
	}

	err = r.zoteroClient.UpdateTags(ctx, item.Key, item.Version, formattedTags, opts.GroupID)
	if err != nil {
		if errors.Is(err, zotero.ErrPreconditionFailed) {
			r.logger.Warn().Str("itemKey", item.Key).Msg("Item modified externally since fetch; skipping tag update (412)")
			return nil
		}
		return fmt.Errorf("failed to update tags in Zotero: %w", err)
	}

	r.logger.Info().Str("itemKey", item.Key).Msg("Successfully updated Zotero tags")
	return nil
}
