package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"jvm/internal/config"
)

// QueryOptions 区分摘要与完整历史，缓存按平台、发行版和查询范围隔离。
type QueryOptions struct {
	Sources     []string
	Major       int
	AllVersions bool
	Refresh     bool
	Offline     bool
}

type SourceReport struct {
	Source   string    `json:"source"`
	Status   string    `json:"status"`
	Count    int       `json:"count"`
	CachedAt time.Time `json:"cached_at,omitempty"`
	Error    string    `json:"error,omitempty"`
	Provider string    `json:"provider,omitempty"`
}

type CatalogResult struct {
	Releases []JavaRelease  `json:"releases"`
	Reports  []SourceReport `json:"reports"`
}

type Catalog struct {
	manager *SourceManager
	load    func() ([]JavaSource, error)
	fetch   func(JavaSource, int, bool) ([]JavaRelease, error)
	now     func() time.Time
}

func NewCatalog() *Catalog {
	m := NewSourceManager()
	return &Catalog{manager: m, load: m.LoadSources, now: time.Now}
}

type catalogCache struct {
	Schema   int           `json:"schema"`
	Updated  time.Time     `json:"updated"`
	Releases []JavaRelease `json:"releases"`
}

const catalogTTL = 6 * time.Hour
const catalogMaxStale = 30 * 24 * time.Hour

// v2 不再将 Azul 元数据的近似大小用作精确字节校验。
const catalogSchema = 2

func (c *Catalog) Query(options QueryOptions) (CatalogResult, error) {
	result := CatalogResult{Releases: []JavaRelease{}, Reports: []SourceReport{}}
	if options.Major < 0 {
		return result, fmt.Errorf("Java major version must be positive")
	}
	if options.Refresh && options.Offline {
		return result, fmt.Errorf("--refresh and --offline cannot be combined")
	}
	all, err := c.load()
	if err != nil {
		return result, err
	}
	requested := map[string]bool{}
	for _, name := range options.Sources {
		requested[name] = true
	}
	selected := []JavaSource{}
	for _, source := range all {
		if len(requested) > 0 && !requested[source.Name] {
			continue
		}
		if !source.Enabled {
			if requested[source.Name] {
				result.Reports = append(result.Reports, SourceReport{Source: source.Name, Status: "disabled", Error: "source is disabled; enable it with jvm sources enable"})
			}
			continue
		}
		selected = append(selected, source)
	}
	for name := range requested {
		found := false
		for _, source := range all {
			if source.Name == name {
				found = true
				break
			}
		}
		if !found {
			return result, fmt.Errorf("unknown source: %s", name)
		}
	}
	if len(selected) == 0 {
		return result, fmt.Errorf("no enabled sources selected; use jvm sources list")
	}
	sort.SliceStable(selected, func(i, j int) bool { return selected[i].Priority < selected[j].Priority })
	root, err := config.GetJVMDir()
	if err != nil {
		return result, err
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	limit := make(chan struct{}, 4)
	for _, source := range selected {
		source := source
		wg.Add(1)
		limit <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-limit }()
			releases, report := c.querySource(source, options, filepath.Join(root, "cache", "catalog"))
			for i := range releases {
				if releases[i].Metadata == nil {
					releases[i].Metadata = map[string]string{}
				}
				releases[i].Metadata["source_priority"] = strconv.Itoa(source.Priority)
			}
			mu.Lock()
			result.Releases = append(result.Releases, releases...)
			result.Reports = append(result.Reports, report)
			mu.Unlock()
		}()
	}
	wg.Wait()
	result.Releases = c.manager.deduplicateReleases(result.Releases)
	priorities := map[string]int{}
	for _, s := range selected {
		priorities[s.Name] = s.Priority
	}
	sort.Slice(result.Reports, func(i, j int) bool {
		if priorities[result.Reports[i].Source] != priorities[result.Reports[j].Source] {
			return priorities[result.Reports[i].Source] < priorities[result.Reports[j].Source]
		}
		return result.Reports[i].Source < result.Reports[j].Source
	})
	if len(result.Releases) == 0 {
		errors := []string{}
		for _, r := range result.Reports {
			if r.Error != "" {
				errors = append(errors, r.Source+": "+r.Error)
			}
		}
		if len(errors) > 0 {
			return result, fmt.Errorf("no catalogue available (%s)", strings.Join(errors, "; "))
		}
	}
	return result, nil
}

func (c *Catalog) querySource(source JavaSource, options QueryOptions, cacheDir string) ([]JavaRelease, SourceReport) {
	report := SourceReport{Source: source.Name}
	key := fmt.Sprintf("%s|%s|%s|%s|%d|%t", source.Name, source.BaseURL, runtime.GOOS, runtime.GOARCH, options.Major, options.AllVersions)
	digest := sha256.Sum256([]byte(key))
	cachePath := filepath.Join(cacheDir, hex.EncodeToString(digest[:])+".json")
	cached, cacheOK := readCatalogCache(cachePath, source.Name, c.now())
	age := c.now().Sub(cached.Updated)
	if cacheOK && (options.Offline || (!options.Refresh && age <= catalogTTL)) {
		if len(cached.Releases) > 0 {
			report.Provider = cached.Releases[0].Metadata["metadata_provider"]
		}
		report.Status, report.Count, report.CachedAt = "cache", len(cached.Releases), cached.Updated
		if age > catalogTTL {
			report.Status = "stale"
			report.Error = "offline cache may no longer reflect available downloads"
		}
		return cached.Releases, report
	}
	if options.Offline {
		report.Status, report.Error = "error", "no usable offline catalogue (run jvm list available --refresh while online)"
		return nil, report
	}
	var releases []JavaRelease
	var err error
	if c.fetch != nil {
		releases, err = c.fetch(source, options.Major, options.AllVersions)
	} else {
		// 限制每个源整个查询时间，而非让多个分页各等待一次超时。
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		manager := *c.manager
		client := *manager.client
		transport := client.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		client.Transport = catalogueTransport{ctx: ctx, base: transport}
		manager.client = &client
		releases, err = manager.fetchVersions(source, options.Major, options.AllVersions)
	}
	if err != nil {
		report.Error = err.Error()
		if cacheOK && age <= catalogMaxStale {
			report.Status, report.Count, report.CachedAt = "stale", len(cached.Releases), cached.Updated
			return cached.Releases, report
		}
		report.Status = "error"
		return nil, report
	}
	report.Status, report.Count, report.CachedAt = "live", len(releases), c.now()
	if len(releases) > 0 {
		report.Provider = releases[0].Metadata["metadata_provider"]
		if reason := releases[0].Metadata["fallback_reason"]; reason != "" {
			report.Error = "using same-vendor fallback metadata: " + reason
		}
	}
	if err := writeCatalogCache(cachePath, catalogCache{Schema: catalogSchema, Updated: report.CachedAt, Releases: releases}); err != nil {
		report.Error = "catalogue fetched, but cache could not be saved: " + err.Error()
	}
	return releases, report
}

type catalogueTransport struct {
	ctx  context.Context
	base http.RoundTripper
}

func (t catalogueTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return t.base.RoundTrip(r.Clone(t.ctx))
}

func readCatalogCache(path, source string, now time.Time) (catalogCache, bool) {
	var value catalogCache
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 32<<20 || json.Unmarshal(data, &value) != nil || value.Schema != catalogSchema || value.Updated.IsZero() || value.Updated.After(now.Add(time.Minute)) || now.Sub(value.Updated) > catalogMaxStale {
		return value, false
	}
	for _, release := range value.Releases {
		if release.Source != source || release.Version == "" || release.MajorVersion < 1 {
			return value, false
		}
	}
	return value, true
}

func writeCatalogCache(path string, value catalogCache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".catalog-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
