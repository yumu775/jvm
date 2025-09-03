package sources

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/fatih/color"
)

// SourceManager 管理不同的 Java 下载源
type SourceManager struct {
	client *http.Client
}

// NewSourceManager 创建一个新的下载源管理器
func NewSourceManager() *SourceManager {
	return &SourceManager{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
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

// GetDefaultSources 获取默认的下载源配置
func (sm *SourceManager) GetDefaultSources() []JavaSource {
	return []JavaSource{
		{
			Name:        "adoptium",
			DisplayName: "Eclipse Adoptium (Temurin)",
			BaseURL:     "https://api.adoptium.net/v3",
			APIType:     "adoptium",
			Enabled:     true,
			Priority:    1,
			Metadata: map[string]string{
				"description": "Eclipse Adoptium provides prebuilt OpenJDK binaries",
				"website":     "https://adoptium.net/",
			},
		},
		{
			Name:        "corretto",
			DisplayName: "Amazon Corretto",
			BaseURL:     "https://corretto.aws",
			APIType:     "corretto",
			Enabled:     true,
			Priority:    2,
			Metadata: map[string]string{
				"description": "Amazon Corretto is a no-cost, multiplatform distribution of OpenJDK",
				"website":     "https://aws.amazon.com/corretto/",
			},
		},
		{
			Name:        "zulu",
			DisplayName: "Azul Zulu",
			BaseURL:     "https://api.azul.com/zulu/download/community/v1.0",
			APIType:     "zulu",
			Enabled:     true,
			Priority:    3,
			Metadata: map[string]string{
				"description": "Azul Zulu builds of OpenJDK",
				"website":     "https://www.azul.com/downloads/",
			},
		},
		{
			Name:        "oracle",
			DisplayName: "Oracle JDK",
			BaseURL:     "https://download.oracle.com/java",
			APIType:     "oracle",
			Enabled:     false, // 默认禁用，因为需要许可证
			Priority:    4,
			Metadata: map[string]string{
				"description": "Oracle JDK (requires license for production use)",
				"website":     "https://www.oracle.com/java/technologies/downloads/",
				"license":     "Oracle Technology Network License Agreement",
			},
		},
		{
			Name:        "graalvm",
			DisplayName: "GraalVM",
			BaseURL:     "https://github.com/graalvm/graalvm-ce-builds/releases",
			APIType:     "graalvm",
			Enabled:     true,
			Priority:    5,
			Metadata: map[string]string{
				"description": "GraalVM Community Edition",
				"website":     "https://www.graalvm.org/",
			},
		},
	}
}

// GetAvailableVersions 从所有启用的源获取可用版本
func (sm *SourceManager) GetAvailableVersions(sources []JavaSource) ([]JavaRelease, error) {
	var allReleases []JavaRelease
	
	for _, source := range sources {
		if !source.Enabled {
			continue
		}
		
		color.Blue("Fetching versions from %s...", source.DisplayName)
		
		releases, err := sm.getVersionsFromSource(source)
		if err != nil {
			color.Yellow("Warning: failed to fetch from %s: %v", source.DisplayName, err)
			continue
		}
		
		// 为每个版本添加源信息
		for i := range releases {
			releases[i].Source = source.Name
			releases[i].Vendor = source.DisplayName
		}
		
		allReleases = append(allReleases, releases...)
	}
	
	// 去重和排序
	allReleases = sm.deduplicateReleases(allReleases)
	
	return allReleases, nil
}

// getVersionsFromSource 从特定源获取版本
func (sm *SourceManager) getVersionsFromSource(source JavaSource) ([]JavaRelease, error) {
	switch source.APIType {
	case "adoptium":
		return sm.getAdoptiumVersions(source)
	case "corretto":
		return sm.getCorrettoVersions(source)
	case "zulu":
		return sm.getZuluVersions(source)
	case "oracle":
		return sm.getOracleVersions(source)
	case "graalvm":
		return sm.getGraalVMVersions(source)
	default:
		return nil, fmt.Errorf("unsupported API type: %s", source.APIType)
	}
}

// getAdoptiumVersions 获取 Adoptium 版本
func (sm *SourceManager) getAdoptiumVersions(source JavaSource) ([]JavaRelease, error) {
	osName, arch := getOSArch()
	
	// 获取可用的主版本号
	majorVersions := []int{8, 11, 17, 21} // 常见的版本
	var releases []JavaRelease
	
	for _, major := range majorVersions {
		url := fmt.Sprintf("%s/binary/latest/%d/ga/%s/%s/jdk/hotspot/normal/eclipse",
			source.BaseURL, major, osName, arch)
		
		// 这里简化处理，实际应该调用 API 获取详细信息
		release := JavaRelease{
			Version:      fmt.Sprintf("%d", major),
			MajorVersion: major,
			FullVersion:  fmt.Sprintf("%d.0.0", major),
			DownloadURL:  url,
			FileName:     sm.buildFileName(fmt.Sprintf("%d.0.0", major), osName, arch, "adoptium"),
			LTS:          sm.isLTSVersion(major),
			Source:       source.Name,
			Vendor:       source.DisplayName,
		}
		
		releases = append(releases, release)
	}
	
	return releases, nil
}

// getCorrettoVersions 获取 Amazon Corretto 版本
func (sm *SourceManager) getCorrettoVersions(source JavaSource) ([]JavaRelease, error) {
	osName, arch := getOSArch()
	
	// Amazon Corretto 支持的版本
	versions := map[int]string{
		8:  "8.392.08.1",
		11: "11.0.21.9.1",
		17: "17.0.9.8.1",
		21: "21.0.1.12.1",
	}
	
	var releases []JavaRelease
	
	for major, version := range versions {
		// 构建 Corretto 下载 URL
		var ext string
		switch osName {
		case "windows":
			ext = "zip"
		case "linux":
			ext = "tar.gz"
		case "mac":
			ext = "tar.gz"
		}
		
		url := fmt.Sprintf("%s/downloads/latest/amazon-corretto-%s-%s-%s-jdk.%s",
			source.BaseURL, version, arch, osName, ext)
		
		release := JavaRelease{
			Version:      fmt.Sprintf("%d", major),
			MajorVersion: major,
			FullVersion:  version,
			DownloadURL:  url,
			FileName:     sm.buildFileName(version, osName, arch, "corretto"),
			LTS:          sm.isLTSVersion(major),
			Source:       source.Name,
			Vendor:       source.DisplayName,
		}
		
		releases = append(releases, release)
	}
	
	return releases, nil
}

// getZuluVersions 获取 Azul Zulu 版本
func (sm *SourceManager) getZuluVersions(source JavaSource) ([]JavaRelease, error) {
	// 简化实现，实际应该调用 Azul API
	osName, arch := getOSArch()
	
	versions := []struct {
		major   int
		version string
	}{
		{8, "8.0.392"},
		{11, "11.0.21"},
		{17, "17.0.9"},
		{21, "21.0.1"},
	}
	
	var releases []JavaRelease
	
	for _, v := range versions {
		release := JavaRelease{
			Version:      fmt.Sprintf("%d", v.major),
			MajorVersion: v.major,
			FullVersion:  v.version,
			DownloadURL:  fmt.Sprintf("%s/bundles/latest/jdk%d.0.0/zulu%s-jdk%s-%s_%s", 
				source.BaseURL, v.major, v.version, v.version, osName, arch),
			FileName:     sm.buildFileName(v.version, osName, arch, "zulu"),
			LTS:          sm.isLTSVersion(v.major),
			Source:       source.Name,
			Vendor:       source.DisplayName,
		}
		
		releases = append(releases, release)
	}
	
	return releases, nil
}

// getOracleVersions 获取 Oracle JDK 版本
func (sm *SourceManager) getOracleVersions(source JavaSource) ([]JavaRelease, error) {
	// Oracle JDK 需要特殊处理，通常需要登录和许可证同意
	color.Yellow("Oracle JDK requires manual download due to license restrictions")
	return []JavaRelease{}, nil
}

// getGraalVMVersions 获取 GraalVM 版本
func (sm *SourceManager) getGraalVMVersions(source JavaSource) ([]JavaRelease, error) {
	// 简化实现，实际应该调用 GitHub API
	versions := []struct {
		major   int
		version string
	}{
		{11, "22.3.3"},
		{17, "22.3.3"},
		{21, "21.0.1"},
	}
	
	var releases []JavaRelease
	
	for _, v := range versions {
		release := JavaRelease{
			Version:      fmt.Sprintf("graalvm-%d", v.major),
			MajorVersion: v.major,
			FullVersion:  v.version,
			DownloadURL:  fmt.Sprintf("%s/download/vm-%s/graalvm-ce-java%d-%s", 
				source.BaseURL, v.version, v.major, runtime.GOOS),
			FileName:     sm.buildFileName(v.version, runtime.GOOS, runtime.GOARCH, "graalvm"),
			LTS:          false, // GraalVM 有自己的发布周期
			Source:       source.Name,
			Vendor:       source.DisplayName,
		}
		
		releases = append(releases, release)
	}
	
	return releases, nil
}

// buildFileName 构建文件名
func (sm *SourceManager) buildFileName(version, osName, arch, vendor string) string {
	var ext string
	switch osName {
	case "windows":
		ext = "zip"
	default:
		ext = "tar.gz"
	}
	
	return fmt.Sprintf("%s-jdk-%s-%s-%s.%s", vendor, version, osName, arch, ext)
}

// isLTSVersion 判断是否为 LTS 版本
func (sm *SourceManager) isLTSVersion(major int) bool {
	ltsVersions := []int{8, 11, 17, 21, 25, 29, 33}
	for _, lts := range ltsVersions {
		if major == lts {
			return true
		}
	}
	return false
}

// deduplicateReleases 去除重复的版本
func (sm *SourceManager) deduplicateReleases(releases []JavaRelease) []JavaRelease {
	seen := make(map[string]JavaRelease)
	
	for _, release := range releases {
		key := fmt.Sprintf("%d-%s", release.MajorVersion, release.Source)
		if existing, exists := seen[key]; !exists || release.FullVersion > existing.FullVersion {
			seen[key] = release
		}
	}
	
	var result []JavaRelease
	for _, release := range seen {
		result = append(result, release)
	}
	
	return result
}

// getOSArch 获取操作系统和架构信息
func getOSArch() (string, string) {
	osName := runtime.GOOS
	arch := runtime.GOARCH
	
	switch osName {
	case "darwin":
		osName = "mac"
	case "windows":
		osName = "windows"
	case "linux":
		osName = "linux"
	}
	
	switch arch {
	case "amd64":
		arch = "x64"
	case "386":
		arch = "x32"
	case "arm64":
		arch = "aarch64"
	}
	
	return osName, arch
}
