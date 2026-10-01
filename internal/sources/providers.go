package sources

import (
	"fmt"
	"strings"
)

func (sm *SourceManager) getVersionsFromSource(source JavaSource) ([]JavaRelease, error) {
	return sm.fetchVersions(source, 0, true)
}
func (sm *SourceManager) fetchVersions(source JavaSource, major int, all bool) ([]JavaRelease, error) {
	switch source.APIType {
	case "adoptium":
		releases, err := sm.fetchAdoptiumVersions(source, major, all)
		if err == nil && len(releases) > 0 {
			return sm.deduplicateReleases(releases), nil
		}
		if err == nil {
			err = fmt.Errorf("no matching packages from Adoptium")
		}
		fallback, fallbackErr := sm.fetchFoojayVersions(source, "temurin", major, all)
		if fallbackErr != nil {
			return nil, fmt.Errorf("Adoptium: %v; same-vendor Foojay fallback: %w", err, fallbackErr)
		}
		for i := range fallback {
			fallback[i].Metadata["fallback_reason"] = err.Error()
		}
		return fallback, nil
	case "zulu":
		return sm.fetchZuluVersions(source, major, all)
	case "corretto":
		return sm.fetchFoojayVersions(source, "corretto", major, all)
	case "graalvm":
		return sm.fetchFoojayVersions(source, "graalvm_community", major, all)
	default:
		return nil, fmt.Errorf("source %s requires manual import", source.Name)
	}
}

// PrepareRelease 仅为最终选中的发行包补齐真实校验和和下载地址。
func (sm *SourceManager) PrepareRelease(release *JavaRelease) error {
	if release == nil {
		return fmt.Errorf("missing release")
	}
	if release.Checksum == "" {
		switch release.Metadata["metadata_provider"] {
		case "azul":
			if err := sm.prepareZuluRelease(release); err != nil {
				return err
			}
		case "foojay":
			if err := sm.prepareFoojayRelease(release); err != nil {
				return err
			}
		default:
			return fmt.Errorf("release has no verified checksum provider")
		}
	}
	osName, _ := getOSArch()
	if !validReleaseMetadata(release.Version, release.MajorVersion, release.MajorVersion, release.FileName, release.Checksum, release.DownloadURL, osName) {
		return fmt.Errorf("invalid download metadata for %s", release.Version)
	}
	return nil
}

func (sm *SourceManager) foojayURL(source JavaSource) string {
	if sm.foojayBase != "" {
		return strings.TrimRight(sm.foojayBase, "/")
	}
	if source.APIType != "adoptium" && source.BaseURL != "" {
		return strings.TrimRight(source.BaseURL, "/")
	}
	return "https://api.foojay.io/disco/v3.0"
}

func latestByMajor(releases []JavaRelease) []JavaRelease {
	sorted := NewSourceManager().deduplicateReleases(releases)
	seen := map[int]bool{}
	result := []JavaRelease{}
	for _, r := range sorted {
		if !seen[r.MajorVersion] {
			seen[r.MajorVersion] = true
			result = append(result, r)
		}
	}
	return result
}
