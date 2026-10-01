package sources

import (
	"fmt"
	"net/url"
	"runtime"
	"strconv"
	"strings"
)

type zuluPackage struct {
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
	JavaVersion []int  `json:"java_version"`
	Build       int    `json:"openjdk_build_number"`
	UUID        string `json:"package_uuid"`
	SHA256      string `json:"sha256_hash"`
	Size        int64  `json:"size"`
	Support     string `json:"support_term"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Bitness     int    `json:"hw_bitness"`
}

func zuluPlatform() (string, string, string, error) {
	osName := runtime.GOOS
	archive := "tar.gz"
	switch osName {
	case "windows":
		archive = "zip"
	case "darwin":
		osName = "macos"
	case "linux":
	default:
		return "", "", "", fmt.Errorf("unsupported Zulu OS: %s", osName)
	}
	arch := ""
	switch runtime.GOARCH {
	case "amd64":
		arch = "x86_64"
	case "386":
		arch = "i686"
	case "arm64":
		arch = "aarch64"
	default:
		return "", "", "", fmt.Errorf("unsupported Zulu architecture: %s", runtime.GOARCH)
	}
	return osName, arch, archive, nil
}
func (sm *SourceManager) fetchZuluVersions(source JavaSource, major int, all bool) ([]JavaRelease, error) {
	osName, arch, archive, err := zuluPlatform()
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(source.BaseURL, "/")
	if base == "" {
		base = "https://api.azul.com/metadata/v1/zulu"
	}
	var releases []JavaRelease
	for page := 1; page <= 1000; page++ {
		q := url.Values{"os": {osName}, "arch": {arch}, "archive_type": {archive}, "java_package_type": {"jdk"}, "javafx_bundled": {"false"}, "crac_supported": {"false"}, "crs_supported": {"false"}, "release_status": {"ga"}, "availability_types": {"CA"}, "page": {strconv.Itoa(page)}, "page_size": {"100"}}
		if major > 0 {
			q.Set("java_version", strconv.Itoa(major))
		}
		if !all {
			q.Set("latest", "true")
		}
		var packages []zuluPackage
		if err = sm.getJSON(base+"/packages/?"+q.Encode(), &packages); err != nil {
			return nil, err
		}
		for _, p := range packages {
			if len(p.JavaVersion) == 0 || p.UUID == "" || p.Name == "" || p.DownloadURL == "" {
				return nil, fmt.Errorf("incomplete Azul package metadata")
			}
			if strings.Contains(p.Name, "-crac-") || strings.Contains(p.Name, "-crs-") || strings.Contains(p.Name, "-fx-") {
				continue
			}
			if major > 0 && p.JavaVersion[0] != major {
				continue
			}
			v := zuluVersion(p)
			releases = append(releases, JavaRelease{Version: v, FullVersion: v, MajorVersion: p.JavaVersion[0], DownloadURL: p.DownloadURL, FileName: p.Name, Checksum: p.SHA256, Source: source.Name, Vendor: source.DisplayName, LTS: isKnownLTS(p.JavaVersion[0]), Metadata: map[string]string{"metadata_provider": "azul", "detail_url": base + "/packages/" + url.PathEscape(p.UUID) + "/", "upstream_version": v, "reported_size": strconv.FormatInt(p.Size, 10)}})
		}
		if len(packages) < 100 {
			break
		}
		if page == 1000 {
			return nil, fmt.Errorf("Azul pagination limit exceeded")
		}
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("no Zulu JDK packages for this platform and version")
	}
	if !all {
		return latestByMajor(releases), nil
	}
	return sm.deduplicateReleases(releases), nil
}
func (sm *SourceManager) prepareZuluRelease(release *JavaRelease) error {
	endpoint := release.Metadata["detail_url"]
	if endpoint == "" {
		return fmt.Errorf("missing Azul package detail URL")
	}
	var p zuluPackage
	if err := sm.getJSON(endpoint, &p); err != nil {
		return err
	}
	if p.Name != release.FileName || p.DownloadURL != release.DownloadURL || zuluVersion(p) != release.Version {
		return fmt.Errorf("Azul package changed since listing")
	}
	osName, _, _, err := zuluPlatform()
	if err != nil {
		return err
	}
	expectedArch := "x86"
	expectedBits := 64
	if runtime.GOARCH == "386" {
		expectedBits = 32
	}
	if runtime.GOARCH == "arm64" {
		expectedArch = "arm"
	}
	if p.OS != osName {
		return fmt.Errorf("Azul package OS mismatch")
	}
	if p.Arch != expectedArch && p.Arch != runtime.GOARCH {
		return fmt.Errorf("Azul package architecture mismatch")
	}
	if p.Bitness != expectedBits {
		return fmt.Errorf("Azul package bitness mismatch")
	}
	release.Checksum = p.SHA256
	// Azul 的 size 按千字节约整，不能作为精确文件长度使用。
	// 实际传输长度由 HTTP Content-Length 检查，文件内容仍强制验证 SHA-256。
	release.Metadata["reported_size"] = strconv.FormatInt(p.Size, 10)
	release.FileSize = 0
	return nil
}

func zuluVersion(p zuluPackage) string {
	parts := make([]string, len(p.JavaVersion))
	for i, n := range p.JavaVersion {
		parts[i] = strconv.Itoa(n)
	}
	v := strings.Join(parts, ".")
	if p.Build > 0 {
		v += "+" + strconv.Itoa(p.Build)
	}
	return v
}
func isKnownLTS(major int) bool {
	switch major {
	case 8, 11, 17, 21, 25:
		return true
	}
	return false
}
