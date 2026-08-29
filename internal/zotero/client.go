package zotero

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/andreassag/zotero-tagger/internal/config"
)

// ErrPreconditionFailed is returned when a Zotero item was modified externally (HTTP 412).
var ErrPreconditionFailed = errors.New("item modified externally (412 Precondition Failed)")

type Client struct {
	httpClient *http.Client
	cfg        config.ZoteroConfig
	baseURL    string
}

func NewClient(cfg config.ZoteroConfig) *Client {
	if cfg.Retries.MaxRetries <= 0 {
		cfg.Retries.MaxRetries = 3
	}
	if cfg.Retries.BackoffBase <= 0 {
		cfg.Retries.BackoffBase = 1.0
	}

	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cfg:        cfg,
		baseURL:    "https://api.zotero.org",
	}
}

func (c *Client) SetBaseURLForTest(url string) {
	c.baseURL = url
}

func (c *Client) getLibraryPrefix(groupID string) string {
	if groupID != "" {
		return fmt.Sprintf("/groups/%s", groupID)
	}
	if c.cfg.LibraryType == "group" && c.cfg.GroupID != "" {
		return fmt.Sprintf("/groups/%s", c.cfg.GroupID)
	}
	return fmt.Sprintf("/users/%s", c.cfg.UserID)
}

func (c *Client) doRequestFactory(reqFactory func() (*http.Request, error)) (*http.Response, error) {
	maxRetries := c.cfg.Retries.MaxRetries
	var resp *http.Response
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			backoff := time.Duration(c.cfg.Retries.BackoffBase*float64(i)) * time.Second
			time.Sleep(backoff)
		}

		req, err := reqFactory()
		if err != nil {
			return nil, err
		}

		req.Header.Set("Zotero-API-Version", "3")
		if c.cfg.APIKey != "" {
			req.Header.Set("Zotero-API-Key", c.cfg.APIKey)
		}

		resp, err = c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if resp != nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfterSec := 5
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if parsed, pErr := strconv.Atoi(ra); pErr == nil {
					retryAfterSec = parsed
				}
			}
			_ = resp.Body.Close()
			time.Sleep(time.Duration(retryAfterSec) * time.Second)
			continue
		}

		if resp.StatusCode >= 500 {
			_ = resp.Body.Close()
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after retries: %w", lastErr)
}

func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	return c.doRequestFactory(func() (*http.Request, error) {
		return req, nil
	})
}

func (c *Client) FetchItems(ctx context.Context, opts FetchOptions) ([]Item, error) {
	prefix := c.getLibraryPrefix(opts.GroupID)
	var endpoint string

	if opts.ItemKey != "" {
		endpoint = fmt.Sprintf("%s/items/%s", prefix, opts.ItemKey)
		req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+endpoint, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.doRequest(req)
		if err != nil {
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to fetch item %s: status %d", opts.ItemKey, resp.StatusCode)
		}

		var item Item
		if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
			return nil, err
		}
		return []Item{item}, nil
	}

	if opts.CollectionKey != "" {
		endpoint = fmt.Sprintf("%s/collections/%s/items/top", prefix, opts.CollectionKey)
	} else {
		endpoint = fmt.Sprintf("%s/items/top", prefix)
	}

	var allItems []Item
	start := 0
	limit := 100

	for {
		req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+endpoint, nil)
		if err != nil {
			return nil, err
		}

		q := req.URL.Query()
		q.Set("start", strconv.Itoa(start))
		q.Set("limit", strconv.Itoa(limit))

		if len(opts.ItemTypes) > 0 {
			q.Set("itemType", strings.Join(opts.ItemTypes, " || "))
		}

		if opts.ExcludeTag != "" && !opts.Reprocess {
			q.Set("tag", "-"+opts.ExcludeTag)
		}

		req.URL.RawQuery = q.Encode()

		resp, err := c.doRequest(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to fetch items: status %d", resp.StatusCode)
		}

		var items []Item
		if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
			_ = resp.Body.Close()
			return nil, err
		}
		_ = resp.Body.Close()

		allowedMap := make(map[string]bool)
		for _, t := range opts.ItemTypes {
			allowedMap[t] = true
		}

		for _, item := range items {
			if len(allowedMap) > 0 && !allowedMap[item.Data.ItemType] {
				continue
			}
			allItems = append(allItems, item)
			if opts.Limit > 0 && len(allItems) >= opts.Limit {
				return allItems, nil
			}
		}

		if len(items) < limit {
			break
		}
		start += limit
	}

	return allItems, nil
}

func (c *Client) DownloadPDF(ctx context.Context, itemKey string, groupID string) (string, error) {
	prefix := c.getLibraryPrefix(groupID)
	childrenEndpoint := fmt.Sprintf("%s/items/%s/children", prefix, itemKey)

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+childrenEndpoint, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch children for item %s: status %d", itemKey, resp.StatusCode)
	}

	var children []Item
	if decErr := json.NewDecoder(resp.Body).Decode(&children); decErr != nil {
		return "", decErr
	}

	var pdfAttachmentKey string
	for _, child := range children {
		if child.Data.ItemType == "attachment" && (strings.HasSuffix(strings.ToLower(child.Data.Filename), ".pdf") || child.Data.ContentType == "application/pdf") {
			pdfAttachmentKey = child.Key
			break
		}
	}

	if pdfAttachmentKey == "" {
		return "", fmt.Errorf("no PDF attachment found for item %s", itemKey)
	}

	fileEndpoint := fmt.Sprintf("%s/items/%s/file", prefix, pdfAttachmentKey)
	fileReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+fileEndpoint, nil)
	if err != nil {
		return "", err
	}

	fileResp, err := c.doRequest(fileReq)
	if err != nil {
		return "", err
	}
	defer func() { _ = fileResp.Body.Close() }()

	if fileResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download PDF file for attachment %s: status %d", pdfAttachmentKey, fileResp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "zotero-*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = tmpFile.Close() }()

	if _, err := io.Copy(tmpFile, fileResp.Body); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to save PDF content: %w", err)
	}

	return tmpFile.Name(), nil
}

func (c *Client) UpdateTags(ctx context.Context, itemKey string, version int, tags []Tag, groupID string) error {
	prefix := c.getLibraryPrefix(groupID)
	endpoint := fmt.Sprintf("%s/items/%s", prefix, itemKey)

	payload := map[string]interface{}{
		"tags": tags,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	reqFactory := func() (*http.Request, error) {
		req, reqErr := http.NewRequestWithContext(ctx, "PATCH", c.baseURL+endpoint, bytes.NewReader(bodyBytes))
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("Content-Type", "application/json")
		if version > 0 {
			req.Header.Set("If-Unmodified-Since-Version", strconv.Itoa(version))
		}
		return req, nil
	}

	resp, err := c.doRequestFactory(reqFactory)
	if err != nil {
		return fmt.Errorf("failed to update tags for item %s: %w", itemKey, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusPreconditionFailed {
		return fmt.Errorf("%w for item %s", ErrPreconditionFailed, itemKey)
	}

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}

	return fmt.Errorf("failed to update tags for item %s: status %d", itemKey, resp.StatusCode)
}
