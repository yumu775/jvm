package sources

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type SourceManager struct {
	client     *http.Client
	foojayBase string
	githubBase string
}

func NewSourceManager() *SourceManager {
	return &SourceManager{client: &http.Client{Timeout: 30 * time.Second}}
}

// JavaSource 表示一个 Java 下载源
type JavaSource struct {
	Name        string            `json:"name"`         // 源名称
	DisplayName string            `json:"display_name"` // 显示名称
	BaseURL     string            `json:"base_url"`     // 基础 URL
	APIType     string            `json:"api_type"`     // API 类型: "adoptium", "corretto", "zulu", "oracle"
	Enabled     bool              `json:"enabled"`      // 是否启用
	Priority    int               `json:"priority"`     // 优先级（数字越小优先级越高）
	Metadata    map[string]string `json:"metadata"`     // 额外元数据
}

// JavaRelease 表示一个 Java 发布版本
type JavaRelease struct {
	Version      string            `json:"version"`       // 版本号
	MajorVersion int               `json:"major_version"` // 主版本号
	FullVersion  string            `json:"full_version"`  // 完整版本号
	DownloadURL  string            `json:"download_url"`  // 下载链接
	FileName     string            `json:"file_name"`     // 文件名
	FileSize     int64             `json:"file_size"`     // 文件大小
	Checksum     string            `json:"checksum"`      // 校验和
	Source       string            `json:"source"`        // 来源
	Vendor       string            `json:"vendor"`        // 供应商
	LTS          bool              `json:"lts"`           // 是否为 LTS 版本
	ReleaseDate  string            `json:"release_date"`  // 发布日期
	Metadata     map[string]string `json:"metadata"`      // 额外信息
}

// GetDefaultSources 仅启用已经实现可靠元数据查询的源。
func (sm *SourceManager) GetDefaultSources() []JavaSource {
	return []JavaSource{
		{Name: "adoptium", DisplayName: "Eclipse Adoptium (Temurin)", BaseURL: "https://api.adoptium.net/v3", APIType: "adoptium", Enabled: true, Priority: 1},
		{Name: "corretto", DisplayName: "Amazon Corretto", BaseURL: "https://api.foojay.io/disco/v3.0", APIType: "corretto", Enabled: true, Priority: 2},
		{Name: "zulu", DisplayName: "Azul Zulu", BaseURL: "https://api.azul.com/metadata/v1/zulu", APIType: "zulu", Enabled: true, Priority: 3},
		{Name: "oracle", DisplayName: "Oracle JDK (manual import)", APIType: "oracle", Priority: 4},
		{Name: "graalvm", DisplayName: "GraalVM Community", BaseURL: "https://api.foojay.io/disco/v3.0", APIType: "graalvm", Enabled: true, Priority: 5},
	}
}
func (sm *SourceManager) GetAvailableVersions(all []JavaSource) ([]JavaRelease, error) {
	var result []JavaRelease
	for _, s := range all {
		if !s.Enabled {
			continue
		}
		if !IsSupportedSource(s) {
			return nil, fmt.Errorf("source %s is not supported for automatic downloads; import manually", s.Name)
		}
		releases, err := sm.getVersionsFromSource(s)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.Name, err)
		}
		result = append(result, releases...)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no releases available from enabled sources")
	}
	return sm.deduplicateReleases(result), nil
}
func (sm *SourceManager) getJSON(endpoint string, target interface{}) error {
	resp, err := sm.client.Get(endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("metadata request returned HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(target)
}

type adoptiumAsset struct {
	Version struct {
		Semver string `json:"semver"`
		Major  int    `json:"major"`
	} `json:"version_data"`
	Binaries []struct {
		Package struct {
			Name     string `json:"name"`
			Link     string `json:"link"`
			Checksum string `json:"checksum"`
			Size     int64  `json:"size"`
		} `json:"package"`
	} `json:"binaries"`
	ReleaseDate string `json:"timestamp"`
}

func (sm *SourceManager) getAdoptiumVersions(source JavaSource) ([]JavaRelease, error) {
	return sm.fetchAdoptiumVersions(source, 0, true)
}
func (sm *SourceManager) fetchAdoptiumVersions(source JavaSource, requestedMajor int, all bool) ([]JavaRelease, error) {
	var available struct {
		Releases []int `json:"available_releases"`
		LTS      []int `json:"available_lts_releases"`
	}
	if err := sm.getJSON(source.BaseURL+"/info/available_releases", &available); err != nil {
		return nil, err
	}
	osName, arch := getOSArch()
	lts := map[int]bool{}
	for _, v := range available.LTS {
		lts[v] = true
	}
	var result []JavaRelease
	for _, major := range available.Releases {
		if requestedMajor > 0 && major != requestedMajor {
			continue
		}
		pageSize := 20
		if !all {
			pageSize = 1
		}
		for page := 0; ; page++ {
			q := url.Values{"architecture": {arch}, "os": {osName}, "image_type": {"jdk"}, "jvm_impl": {"hotspot"}, "heap_size": {"normal"}, "vendor": {"eclipse"}, "page_size": {strconv.Itoa(pageSize)}, "page": {strconv.Itoa(page)}}
			endpoint := fmt.Sprintf("%s/assets/feature_releases/%d/ga?%s", source.BaseURL, major, q.Encode())
			var assets []adoptiumAsset
			// 某些平台没有对应发行包，API 会返回 404。
			resp, err := sm.client.Get(endpoint)
			if err != nil {
				return nil, err
			}
			if resp.StatusCode == http.StatusNotFound {
				resp.Body.Close()
				break
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return nil, fmt.Errorf("metadata request returned HTTP %d", resp.StatusCode)
			}
			err = json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&assets)
			resp.Body.Close()
			if err != nil {
				return nil, err
			}
			for _, a := range assets {
				for _, b := range a.Binaries {
					p := b.Package
					if !validReleaseMetadata(a.Version.Semver, a.Version.Major, major, p.Name, p.Checksum, p.Link, osName) || p.Size <= 0 {
						return nil, fmt.Errorf("incomplete release metadata")
					}
					result = append(result, JavaRelease{Version: a.Version.Semver, FullVersion: a.Version.Semver, MajorVersion: a.Version.Major, DownloadURL: p.Link, FileName: p.Name, FileSize: p.Size, Checksum: p.Checksum, Source: source.Name, Vendor: source.DisplayName, LTS: lts[major], ReleaseDate: a.ReleaseDate, Metadata: map[string]string{"metadata_provider": "adoptium"}})
				}
			}
			if !all || len(assets) < pageSize {
				break
			}
			if page >= 999 {
				return nil, fmt.Errorf("release pagination limit exceeded")
			}
		}
	}
	return result, nil
}

// validReleaseMetadata 拒绝安装器、越界文件名及不完整元数据。
func validReleaseMetadata(version string, reportedMajor, requestedMajor int, name, checksum, link, osName string) bool {
	if !regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)*(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`).MatchString(version) {
		return false
	}
	majorText := strings.FieldsFunc(version, func(r rune) bool { return r == '.' || r == '+' })[0]
	major, err := strconv.Atoi(majorText)
	if err != nil || major != reportedMajor || major != requestedMajor {
		return false
	}
	if filepath.Base(name) != name || strings.ContainsAny(name, `/\:`) {
		return false
	}
	ext := ".tar.gz"
	if osName == "windows" {
		ext = ".zip"
	}
	if !strings.HasSuffix(strings.ToLower(name), ext) {
		return false
	}
	digest, err := hex.DecodeString(checksum)
	if err != nil || len(digest) != 32 {
		return false
	}
	u, err := url.Parse(link)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

// CompareVersions 按数字段比较版本，确保 17.0.10 排在 17.0.9 之后。
func CompareVersions(a, b string) int {
	split := func(s string) []string {
		return strings.FieldsFunc(s, func(r rune) bool { return r == '.' || r == '+' || r == '_' || r == '-' })
	}
	x, y := split(a), split(b)
	n := len(x)
	if len(y) > n {
		n = len(y)
	}
	for i := 0; i < n; i++ {
		u, v := "0", "0"
		if i < len(x) {
			u = x[i]
		}
		if i < len(y) {
			v = y[i]
		}
		nu, eu := strconv.Atoi(u)
		nv, ev := strconv.Atoi(v)
		if eu == nil && ev == nil {
			if nu < nv {
				return -1
			}
			if nu > nv {
				return 1
			}
		} else {
			if u < v {
				return -1
			}
			if u > v {
				return 1
			}
		}
	}
	return 0
}
func (sm *SourceManager) deduplicateReleases(releases []JavaRelease) []JavaRelease {
	seen := map[string]bool{}
	result := []JavaRelease{}
	for _, r := range releases {
		key := r.Source + "|" + r.FullVersion
		if !seen[key] {
			seen[key] = true
			result = append(result, r)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		c := CompareVersions(result[i].FullVersion, result[j].FullVersion)
		if c != 0 {
			return c > 0
		}
		return result[i].Source < result[j].Source
	})
	return result
}
func getOSArch() (string, string) {
	o, a := runtime.GOOS, runtime.GOARCH
	if o == "darwin" {
		o = "mac"
	}
	switch a {
	case "amd64":
		a = "x64"
	case "386":
		a = "x86"
	case "arm64":
		a = "aarch64"
	}
	return o, a
}
