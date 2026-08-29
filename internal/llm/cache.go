package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type DiskCache struct {
	cacheDir string
	mu       sync.Mutex
}

type CachedResponse struct {
	Response string `json:"response"`
}

func NewDiskCache() *DiskCache {
	userCache, err := os.UserCacheDir()
	var cacheDir string
	if err != nil {
		cacheDir = ".zotero-tagger-cache"
	} else {
		cacheDir = filepath.Join(userCache, "zotero-tagger", "llm")
	}
	_ = os.MkdirAll(cacheDir, 0750)

	return &DiskCache{
		cacheDir: cacheDir,
	}
}

func HashPrompt(model, systemPrompt, userPrompt string) string {
	hasher := sha256.New()
	hasher.Write([]byte(model))
	hasher.Write([]byte("|"))
	hasher.Write([]byte(systemPrompt))
	hasher.Write([]byte("|"))
	hasher.Write([]byte(userPrompt))
	return hex.EncodeToString(hasher.Sum(nil))
}

func HashPDFPrompt(model, systemPrompt, userPrompt, pdfPath string) (string, error) {
	data, err := os.ReadFile(filepath.Clean(pdfPath))
	if err != nil {
		return "", err
	}
	pdfHash := sha256.Sum256(data)

	hasher := sha256.New()
	hasher.Write([]byte(model))
	hasher.Write([]byte("|"))
	hasher.Write([]byte(systemPrompt))
	hasher.Write([]byte("|"))
	hasher.Write([]byte(userPrompt))
	hasher.Write([]byte("|"))
	hasher.Write(pdfHash[:])
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (c *DiskCache) Get(promptHash string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	filePath := filepath.Join(c.cacheDir, filepath.Clean(promptHash)+".json")
	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return "", false
	}

	var cached CachedResponse
	if err := json.Unmarshal(data, &cached); err != nil {
		return "", false
	}

	return cached.Response, true
}

func (c *DiskCache) Set(promptHash string, response string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cached := CachedResponse{Response: response}
	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(c.cacheDir, filepath.Clean(promptHash)+".json")
	return os.WriteFile(filePath, data, 0600)
}
