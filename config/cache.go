package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type CacheEntry struct {
	FullID string `json:"full_id"`
	OrgID  string `json:"org_id,omitempty"`
	Name   string `json:"name,omitempty"`
	Type   string `json:"type"` // "node" or "org"
}

type IDCache struct {
	Entries map[string]CacheEntry `json:"entries"`
}

func cacheFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".yertle", "id-cache.json")
}

// LoadCache reads the ID cache from ~/.yertle/id-cache.json.
// Returns an empty cache if the file doesn't exist or is corrupt.
func LoadCache() *IDCache {
	cache := &IDCache{Entries: make(map[string]CacheEntry)}

	data, err := os.ReadFile(cacheFilePath())
	if err != nil {
		return cache // File missing or unreadable — start fresh
	}

	if err := json.Unmarshal(data, cache); err != nil {
		// Corrupt cache — start fresh rather than failing
		return &IDCache{Entries: make(map[string]CacheEntry)}
	}
	if cache.Entries == nil {
		cache.Entries = make(map[string]CacheEntry)
	}
	return cache
}

// Save writes the cache to ~/.yertle/id-cache.json.
func (c *IDCache) Save() error {
	path := cacheFilePath()
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Put stores a mapping from short ID (first 8 chars) to full ID.
func (c *IDCache) Put(fullID, orgID, name, idType string) {
	short := ShortID(fullID)
	if short == "" {
		return
	}
	c.Entries[short] = CacheEntry{
		FullID: fullID,
		OrgID:  orgID,
		Name:   name,
		Type:   idType,
	}
}

// Resolve looks up an input string and returns the full ID and org ID.
// If the input is already a full UUID (contains "-" or is 32+ chars), it's returned as-is.
// Returns (fullID, orgID, found).
func (c *IDCache) Resolve(input string) (string, string, bool) {
	if isFullUUID(input) {
		return input, "", false
	}

	if entry, ok := c.Entries[input]; ok {
		return entry.FullID, entry.OrgID, true
	}

	return input, "", false
}

// ShortID returns the first 8 characters of a UUID string.
func ShortID(fullID string) string {
	// Strip dashes and take first 8 hex chars
	clean := strings.ReplaceAll(fullID, "-", "")
	if len(clean) < 8 {
		return clean
	}
	return clean[:8]
}

func isFullUUID(s string) bool {
	return strings.Contains(s, "-") || len(s) >= 32
}
