package sources

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

type discoPackage struct {
	ID           string   `json:"id"`
	Distribution string   `json:"distribution"`
	Major        int      `json:"major_version"`
	JavaVersion  string   `json:"java_version"`
	Filename     string   `json:"filename"`
	Size         int64    `json:"size"`
	Support      string   `json:"term_of_support"`
	OS           string   `json:"operating_system"`
	Architecture string   `json:"architecture"`
	Archive      string   `json:"archive_type"`
	PackageType  string   `json:"package_type"`
	FX           bool     `json:"javafx_bundled"`
	Status       string   `json:"release_status"`
	Features     []string `json:"feature"`
	Links        struct {
		Info     string `json:"pkg_info_uri"`
		Redirect string `json:"pkg_download_redirect"`
	} `json:"links"`
}

func (sm *SourceManager) fetchFoojayVersions(source JavaSource, distribution string, major int, all bool) ([]JavaRelease, error) {
	osName, arch := getOSArch()
	if osName == "mac" {
		osName = "macos"
	}
	archive := "tar.gz"
	if runtime.GOOS == "windows" {
		archive = "zip"
	}
	q := url.Values{"distribution": {distribution}, "architecture": {arch}, "operating_system": {osName}, "package_type": {"jdk"}, "archive_type": {archive}, "directly_downloadable": {"true"}, "release_status": {"ga"}, "javafx_bundled": {"false"}}
	if runtime.GOOS == "linux" {
		q.Set("lib_c_type", "glibc")
	}
	if major > 0 {
		q.Set("version", strconv.Itoa(major))
	}
	if !all {
		q.Set("latest", "available")
	}
	var response struct {
		Result  []discoPackage `json:"result"`
		Message string         `json:"message"`
	}
	if err := sm.getJSON(sm.foojayURL(source)+"/packages?"+q.Encode(), &response); err != nil {
		return nil, err
	}
	var releases []JavaRelease
	for _, p := range response.Result {
		if p.Distribution != distribution || p.OS != osName || p.Architecture != arch || p.Archive != archive || p.PackageType != "jdk" || p.FX || p.Status != "ga" {
			continue
		}
		if major > 0 && p.Major != major {
			continue
		}
		if len(p.Features) > 0 {
			continue
		}
		if p.ID == "" || p.JavaVersion == "" || p.Filename == "" {
			return nil, fmt.Errorf("incomplete Foojay package metadata")
		}
		detail := sm.foojayURL(source) + "/ids/" + url.PathEscape(p.ID)
		releases = append(releases, JavaRelease{Version: p.JavaVersion, FullVersion: p.JavaVersion, MajorVersion: p.Major, FileName: p.Filename, FileSize: p.Size, DownloadURL: p.Links.Redirect, Source: source.Name, Vendor: source.DisplayName, LTS: p.Support == "lts", Metadata: map[string]string{"metadata_provider": "foojay", "detail_url": detail, "distribution": distribution, "upstream_version": p.JavaVersion}})
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("no %s packages from Foojay for this platform and version", distribution)
	}
	if !all {
		return latestByMajor(releases), nil
	}
	return sm.deduplicateReleases(releases), nil
}
func (sm *SourceManager) prepareFoojayRelease(release *JavaRelease) error {
	endpoint := release.Metadata["detail_url"]
	if endpoint == "" {
		return fmt.Errorf("missing Foojay detail URL")
	}
	var response struct {
		Result []struct {
			Filename    string `json:"filename"`
			URL         string `json:"direct_download_uri"`
			Checksum    string `json:"checksum"`
			Type        string `json:"checksum_type"`
			ChecksumURI string `json:"checksum_uri"`
		} `json:"result"`
	}
	if err := sm.getJSON(endpoint, &response); err != nil {
		return err
	}
	if len(response.Result) != 1 {
		return fmt.Errorf("unexpected Foojay package detail result")
	}
	p := response.Result[0]
	if p.Filename != release.FileName {
		return fmt.Errorf("Foojay package identity mismatch")
	}
	checksumType := strings.ToLower(strings.ReplaceAll(p.Type, "-", ""))
	if p.Checksum == "" && checksumType == "sha256" && p.ChecksumURI != "" {
		checksum, err := sm.readPublishedChecksum(p.ChecksumURI, release.FileName)
		if err != nil {
			return err
		}
		p.Checksum = checksum
	}
	if p.Checksum == "" && release.Metadata["distribution"] == "corretto" {
		checksum, err := sm.correttoReleaseChecksum(release, p.URL)
		if err != nil {
			return err
		}
		p.Checksum = checksum
		checksumType = "sha256"
	}
	if checksumType != "sha256" || p.Checksum == "" {
		return fmt.Errorf("Foojay package has no verifiable SHA-256 checksum")
	}
	release.DownloadURL = p.URL
	release.Checksum = p.Checksum
	return nil
}

func (sm *SourceManager) readPublishedChecksum(endpoint, filename string) (string, error) {
	response, err := sm.client.Get(endpoint)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum request returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return "", err
	}
	if raw := strings.TrimSpace(string(data)); regexp.MustCompile(`^[a-fA-F0-9]{64}$`).MatchString(raw) {
		return strings.ToLower(raw), nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || !regexp.MustCompile(`^[a-fA-F0-9]{64}$`).MatchString(fields[0]) {
			continue
		}
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == filename {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("published checksum does not contain a SHA-256 for %s", filename)
}

// Corretto 的下载详情没有内嵌校验和，官方 GitHub 发布表提供逐文件 SHA-256。
func (sm *SourceManager) correttoReleaseChecksum(release *JavaRelease, downloadURL string) (string, error) {
	u, err := url.Parse(downloadURL)
	if err != nil || u.Scheme != "https" || u.Host != "corretto.aws" {
		return "", fmt.Errorf("invalid Corretto resource URL")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "downloads" || parts[1] != "resources" || parts[3] != release.FileName {
		return "", fmt.Errorf("Corretto resource is not version-pinned")
	}
	base := sm.githubBase
	if base == "" {
		base = "https://api.github.com"
	}
	endpoint := fmt.Sprintf("%s/repos/corretto/corretto-%d/releases/tags/%s", strings.TrimRight(base, "/"), release.MajorVersion, url.PathEscape(parts[2]))
	var result struct {
		Body string `json:"body"`
	}
	if err = sm.getJSON(endpoint, &result); err != nil {
		return "", err
	}
	for _, line := range strings.Split(result.Body, "\n") {
		if !strings.Contains(line, "]("+downloadURL+")") {
			continue
		}
		if checksum := regexp.MustCompile(`\b[a-fA-F0-9]{64}\b`).FindString(line); checksum != "" {
			return strings.ToLower(checksum), nil
		}
	}
	return "", fmt.Errorf("official Corretto release has no SHA-256 for %s", release.FileName)
}
