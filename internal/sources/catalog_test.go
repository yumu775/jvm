package sources

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func testCatalog(t *testing.T) *Catalog {
	t.Helper()
	t.Setenv("JVM_HOME", t.TempDir())
	c := NewCatalog()
	c.now = func() time.Time { return time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC) }
	c.load = func() ([]JavaSource, error) {
		return []JavaSource{{Name: "adoptium", Enabled: true}, {Name: "zulu", Enabled: true}}, nil
	}
	return c
}

func fakeRelease(source string) []JavaRelease {
	return []JavaRelease{{Version: "17.0.12+7", FullVersion: "17.0.12+7", MajorVersion: 17, Source: source}}
}

func TestCatalogPartialFailureKeepsHealthyDistribution(t *testing.T) {
	c := testCatalog(t)
	c.fetch = func(s JavaSource, _ int, _ bool) ([]JavaRelease, error) {
		if s.Name == "adoptium" {
			return nil, errors.New("metadata endpoint unavailable")
		}
		return fakeRelease(s.Name), nil
	}
	result, err := c.Query(QueryOptions{})
	if err != nil || len(result.Releases) != 1 || result.Releases[0].Source != "zulu" {
		t.Fatalf("lost healthy source: %+v %v", result, err)
	}
	if len(result.Reports) != 2 || result.Reports[0].Status != "error" || result.Reports[1].Status != "live" {
		t.Fatalf("missing partial failure status: %+v", result.Reports)
	}
}

func TestCatalogCacheOfflineAndExplicitRefresh(t *testing.T) {
	c := testCatalog(t)
	var calls atomic.Int32
	c.fetch = func(s JavaSource, _ int, _ bool) ([]JavaRelease, error) {
		calls.Add(1)
		return fakeRelease(s.Name), nil
	}
	options := QueryOptions{Sources: []string{"adoptium"}, Major: 17}
	if _, err := c.Query(options); err != nil {
		t.Fatal(err)
	}
	result, err := c.Query(options)
	if err != nil || calls.Load() != 1 || result.Reports[0].Status != "cache" {
		t.Fatalf("cache miss: %+v %v calls=%d", result, err, calls.Load())
	}
	options.Offline = true
	if _, err := c.Query(options); err != nil || calls.Load() != 1 {
		t.Fatalf("offline touched network: %v", err)
	}
	options.Offline = false
	options.Refresh = true
	if _, err := c.Query(options); err != nil || calls.Load() != 2 {
		t.Fatalf("refresh did not fetch: %v", err)
	}
}

func TestCatalogStaleCacheIsExplicitAndExpires(t *testing.T) {
	c := testCatalog(t)
	initial := c.now()
	c.fetch = func(s JavaSource, _ int, _ bool) ([]JavaRelease, error) { return fakeRelease(s.Name), nil }
	options := QueryOptions{Sources: []string{"adoptium"}}
	if _, err := c.Query(options); err != nil {
		t.Fatal(err)
	}
	c.now = func() time.Time { return initial.Add(24 * time.Hour) }
	c.fetch = func(JavaSource, int, bool) ([]JavaRelease, error) { return nil, errors.New("network down") }
	result, err := c.Query(options)
	if err != nil || result.Reports[0].Status != "stale" || result.Reports[0].Error != "network down" {
		t.Fatalf("unmarked stale data: %+v %v", result, err)
	}
	c.now = func() time.Time { return initial.Add(31 * 24 * time.Hour) }
	if _, err := c.Query(options); err == nil {
		t.Fatal("expired stale data used")
	}
}

func TestCatalogCacheDoesNotMixQueryScopes(t *testing.T) {
	c := testCatalog(t)
	c.fetch = func(s JavaSource, _ int, _ bool) ([]JavaRelease, error) { return fakeRelease(s.Name), nil }
	if _, err := c.Query(QueryOptions{Sources: []string{"adoptium"}, Major: 17}); err != nil {
		t.Fatal(err)
	}
	for _, options := range []QueryOptions{
		{Sources: []string{"adoptium"}, Major: 21, Offline: true},
		{Sources: []string{"adoptium"}, Major: 17, AllVersions: true, Offline: true},
		{Sources: []string{"zulu"}, Major: 17, Offline: true},
	} {
		if _, err := c.Query(options); err == nil {
			t.Fatalf("used cache for different query: %+v", options)
		}
	}
}

func TestCatalogRejectsConflictingOrDisabledQueries(t *testing.T) {
	c := testCatalog(t)
	c.load = func() ([]JavaSource, error) { return []JavaSource{{Name: "adoptium", Enabled: false}}, nil }
	c.fetch = func(JavaSource, int, bool) ([]JavaRelease, error) { t.Error("unexpected fetch"); return nil, nil }
	for _, options := range []QueryOptions{{Refresh: true, Offline: true}, {Sources: []string{"adoptium"}}, {Sources: []string{"unknown"}}} {
		if _, err := c.Query(options); err == nil {
			t.Fatalf("accepted %+v", options)
		}
	}
}
