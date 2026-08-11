package catalog

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/faroos/faroos/internal/appcatalog"
)

// refreshInterval bounds how stale the imported catalog is allowed to get
// before Store.NeedsRefresh suggests fetching again. Unraid's feed doesn't
// change fast enough to need more than daily.
const refreshInterval = 24 * time.Hour

// Bump this when the imported cache schema gains data that cannot be
// reconstructed from older cache files. Version 2 adds container arguments.
const cacheVersion = 2

type cacheFile struct {
	Version   int              `json:"version"`
	FetchedAt time.Time        `json:"fetchedAt"`
	Apps      []appcatalog.App `json:"apps"`
}

// Store holds the merged app catalog (FaroOS's curated list + whatever was
// last imported from Unraid CA) in memory, persisted to a JSON file on disk
// so a server restart doesn't need to re-download ~20MB before the App
// Store has anything imported to show.
type Store struct {
	mu        sync.RWMutex
	imported  []appcatalog.App
	fetchedAt time.Time
	version   int
	cachePath string
}

func NewStore(cachePath string) *Store {
	return &Store{cachePath: cachePath}
}

// LoadCache reads a previously-saved import from disk, if any. Safe to call
// even if the file doesn't exist yet (first run).
func (s *Store) LoadCache() error {
	data, err := os.ReadFile(s.cachePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return err
	}
	s.mu.Lock()
	s.imported = cf.Apps
	s.fetchedAt = cf.FetchedAt
	s.version = cf.Version
	s.mu.Unlock()
	return nil
}

func (s *Store) saveCache() error {
	s.mu.RLock()
	cf := cacheFile{Version: cacheVersion, FetchedAt: s.fetchedAt, Apps: s.imported}
	s.mu.RUnlock()

	data, err := json.Marshal(cf)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(s.cachePath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := s.cachePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.cachePath)
}

// NeedsRefresh reports whether the imported catalog is missing or stale
// enough to be worth re-fetching.
func (s *Store) NeedsRefresh() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.imported) == 0 || s.version != cacheVersion || time.Since(s.fetchedAt) > refreshInterval
}

// Refresh fetches the latest Unraid CA catalog and replaces the imported
// set. Can take a while (multi-MB download) — callers should run it in a
// goroutine rather than blocking a request on it.
func (s *Store) Refresh(ctx context.Context) error {
	apps, err := FetchUnraidCatalog(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.imported = apps
	s.fetchedAt = time.Now()
	s.version = cacheVersion
	s.mu.Unlock()

	if err := s.saveCache(); err != nil {
		log.Printf("catalog: failed to save cache: %v", err)
	}
	return nil
}

// RefreshInBackground kicks off Refresh without blocking, logging the
// outcome — used at server startup so a fresh install gets a populated App
// Store without holding up boot on a ~20MB download.
func (s *Store) RefreshInBackground() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		start := time.Now()
		if err := s.Refresh(ctx); err != nil {
			log.Printf("catalog: background refresh failed: %v", err)
			return
		}
		log.Printf("catalog: imported %d apps from Unraid CA in %s", len(s.List())-len(appcatalog.Curated), time.Since(start).Round(time.Second))
	}()
}

// List returns the curated apps followed by the imported catalog.
func (s *Store) List() []appcatalog.App {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]appcatalog.App, 0, len(appcatalog.Curated)+len(s.imported))
	out = append(out, appcatalog.Curated...)
	out = append(out, s.imported...)
	return out
}

func (s *Store) Find(id string) (appcatalog.App, bool) {
	for _, a := range appcatalog.Curated {
		if a.ID == id {
			return a, true
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, a := range s.imported {
		if a.ID == id {
			return a, true
		}
	}
	return appcatalog.App{}, false
}

// Categories returns the distinct, sorted-by-frequency category names
// present in the current catalog, for the App Store's filter pills.
func (s *Store) Categories() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := make(map[string]int)
	order := make([]string, 0)
	for _, a := range appcatalog.Curated {
		if _, ok := counts[a.Category]; !ok {
			order = append(order, a.Category)
		}
		counts[a.Category]++
	}
	for _, a := range s.imported {
		if _, ok := counts[a.Category]; !ok {
			order = append(order, a.Category)
		}
		counts[a.Category]++
	}
	return order
}

func (s *Store) LastUpdated() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fetchedAt
}
