package alias

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"jvm/internal/sources"
)

// Resolver 负责解析版本别名
type Resolver struct {
	sourceManager *sources.SourceManager
}

// NewResolver 创建一个新的别名解析器
func NewResolver() *Resolver {
	return &Resolver{
		sourceManager: sources.NewSourceManager(),
	}
}

// VersionAlias 表示一个版本别名
type VersionAlias struct {
	Alias       string `json:"alias"`       // 别名，如 "latest", "lts"
	Description string `json:"description"` // 描述
	Pattern     string `json:"pattern"`     // 匹配模式
}

// GetSupportedAliases 获取支持的别名列表
func (r *Resolver) GetSupportedAliases() []VersionAlias {
	return []VersionAlias{
		{
			Alias:       "latest",
			Description: "最新的稳定版本",
			Pattern:     "highest_version",
		},
		{
			Alias:       "lts",
			Description: "最新的 LTS (长期支持) 版本",
			Pattern:     "latest_lts",
		},
		{
			Alias:       "lts-1",
			Description: "上一个 LTS 版本",
			Pattern:     "previous_lts",
		},
		{
			Alias:       "stable",
			Description: "最新的稳定版本 (等同于 latest)",
			Pattern:     "highest_version",
		},
		{
			Alias:       "current",
			Description: "当前推荐版本 (通常是最新 LTS)",
			Pattern:     "latest_lts",
		},
	}
}

// ResolveAlias 解析版本别名为具体版本
func (r *Resolver) ResolveAlias(alias string, availableReleases []sources.JavaRelease, preferredSource string) (*sources.JavaRelease, error) {
	alias = strings.ToLower(strings.TrimSpace(alias))
	
	// 检查是否是数字版本（直接返回）
	if r.isNumericVersion(alias) {
		return r.findExactVersion(alias, availableReleases, preferredSource)
	}
	
	// 解析别名
	switch alias {
	case "latest", "stable":
		return r.resolveLatest(availableReleases, preferredSource)
	case "lts", "current":
		return r.resolveLatestLTS(availableReleases, preferredSource)
	case "lts-1":
		return r.resolvePreviousLTS(availableReleases, preferredSource)
	default:
		// 检查是否是特殊格式的别名
		if strings.HasPrefix(alias, "lts-") {
			return r.resolveLTSOffset(alias, availableReleases, preferredSource)
		}
		if strings.HasSuffix(alias, "-latest") {
			return r.resolveSourceLatest(alias, availableReleases)
		}
		
		return nil, fmt.Errorf("unknown alias: %s", alias)
	}
}

// isNumericVersion 检查是否是数字版本
func (r *Resolver) isNumericVersion(version string) bool {
	// 检查是否以数字开头
	if len(version) == 0 {
		return false
	}
	
	// 简单的数字版本检查
	parts := strings.Split(version, ".")
	if len(parts) == 0 {
		return false
	}
	
	_, err := strconv.Atoi(parts[0])
	return err == nil
}

// findExactVersion 查找精确版本匹配
func (r *Resolver) findExactVersion(version string, releases []sources.JavaRelease, preferredSource string) (*sources.JavaRelease, error) {
	var candidates []sources.JavaRelease
	
	// 尝试精确匹配
	for _, release := range releases {
		if release.Version == version || release.FullVersion == version {
			candidates = append(candidates, release)
		}
	}
	
	// 如果没有精确匹配，尝试主版本号匹配
	if len(candidates) == 0 {
		if majorVersion, err := strconv.Atoi(version); err == nil {
			for _, release := range releases {
				if release.MajorVersion == majorVersion {
					candidates = append(candidates, release)
				}
			}
		}
	}
	
	if len(candidates) == 0 {
		return nil, fmt.Errorf("version %s not found", version)
	}
	
	// 如果有多个候选版本，选择最佳匹配
	return r.selectBestCandidate(candidates, preferredSource), nil
}

// resolveLatest 解析 latest 别名
func (r *Resolver) resolveLatest(releases []sources.JavaRelease, preferredSource string) (*sources.JavaRelease, error) {
	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases available")
	}
	
	// 按主版本号排序，选择最高版本
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].MajorVersion > releases[j].MajorVersion
	})
	
	// 获取最高主版本号的所有版本
	highestMajor := releases[0].MajorVersion
	var candidates []sources.JavaRelease
	
	for _, release := range releases {
		if release.MajorVersion == highestMajor {
			candidates = append(candidates, release)
		}
	}
	
	return r.selectBestCandidate(candidates, preferredSource), nil
}

// resolveLatestLTS 解析最新 LTS 版本
func (r *Resolver) resolveLatestLTS(releases []sources.JavaRelease, preferredSource string) (*sources.JavaRelease, error) {
	var ltsReleases []sources.JavaRelease
	
	// 筛选 LTS 版本
	for _, release := range releases {
		if release.LTS {
			ltsReleases = append(ltsReleases, release)
		}
	}
	
	if len(ltsReleases) == 0 {
		return nil, fmt.Errorf("no LTS releases available")
	}
	
	// 按主版本号排序，选择最新的 LTS
	sort.Slice(ltsReleases, func(i, j int) bool {
		return ltsReleases[i].MajorVersion > ltsReleases[j].MajorVersion
	})
	
	// 获取最新 LTS 主版本的所有版本
	latestLTSMajor := ltsReleases[0].MajorVersion
	var candidates []sources.JavaRelease
	
	for _, release := range ltsReleases {
		if release.MajorVersion == latestLTSMajor {
			candidates = append(candidates, release)
		}
	}
	
	return r.selectBestCandidate(candidates, preferredSource), nil
}

// resolvePreviousLTS 解析上一个 LTS 版本
func (r *Resolver) resolvePreviousLTS(releases []sources.JavaRelease, preferredSource string) (*sources.JavaRelease, error) {
	var ltsReleases []sources.JavaRelease
	
	// 筛选 LTS 版本
	for _, release := range releases {
		if release.LTS {
			ltsReleases = append(ltsReleases, release)
		}
	}
	
	if len(ltsReleases) < 2 {
		return nil, fmt.Errorf("not enough LTS releases available")
	}
	
	// 按主版本号排序
	sort.Slice(ltsReleases, func(i, j int) bool {
		return ltsReleases[i].MajorVersion > ltsReleases[j].MajorVersion
	})
	
	// 获取第二新的 LTS 版本
	previousLTSMajor := ltsReleases[1].MajorVersion
	var candidates []sources.JavaRelease
	
	for _, release := range ltsReleases {
		if release.MajorVersion == previousLTSMajor {
			candidates = append(candidates, release)
		}
	}
	
	return r.selectBestCandidate(candidates, preferredSource), nil
}

// resolveLTSOffset 解析 LTS 偏移别名（如 lts-2, lts-3）
func (r *Resolver) resolveLTSOffset(alias string, releases []sources.JavaRelease, preferredSource string) (*sources.JavaRelease, error) {
	// 解析偏移量
	parts := strings.Split(alias, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid LTS offset format: %s", alias)
	}
	
	offset, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid LTS offset number: %s", parts[1])
	}
	
	var ltsReleases []sources.JavaRelease
	
	// 筛选 LTS 版本
	for _, release := range releases {
		if release.LTS {
			ltsReleases = append(ltsReleases, release)
		}
	}
	
	if len(ltsReleases) <= offset {
		return nil, fmt.Errorf("not enough LTS releases for offset %d", offset)
	}
	
	// 按主版本号排序
	sort.Slice(ltsReleases, func(i, j int) bool {
		return ltsReleases[i].MajorVersion > ltsReleases[j].MajorVersion
	})
	
	// 获取指定偏移的 LTS 版本
	targetLTSMajor := ltsReleases[offset].MajorVersion
	var candidates []sources.JavaRelease
	
	for _, release := range ltsReleases {
		if release.MajorVersion == targetLTSMajor {
			candidates = append(candidates, release)
		}
	}
	
	return r.selectBestCandidate(candidates, preferredSource), nil
}

// resolveSourceLatest 解析特定源的最新版本（如 adoptium-latest）
func (r *Resolver) resolveSourceLatest(alias string, releases []sources.JavaRelease) (*sources.JavaRelease, error) {
	sourceName := strings.TrimSuffix(alias, "-latest")
	
	var sourceReleases []sources.JavaRelease
	
	// 筛选特定源的版本
	for _, release := range releases {
		if release.Source == sourceName {
			sourceReleases = append(sourceReleases, release)
		}
	}
	
	if len(sourceReleases) == 0 {
		return nil, fmt.Errorf("no releases found for source: %s", sourceName)
	}
	
	// 按主版本号排序，选择最高版本
	sort.Slice(sourceReleases, func(i, j int) bool {
		return sourceReleases[i].MajorVersion > sourceReleases[j].MajorVersion
	})
	
	return &sourceReleases[0], nil
}

// selectBestCandidate 从候选版本中选择最佳匹配
func (r *Resolver) selectBestCandidate(candidates []sources.JavaRelease, preferredSource string) *sources.JavaRelease {
	if len(candidates) == 0 {
		return nil
	}
	
	if len(candidates) == 1 {
		return &candidates[0]
	}
	
	// 如果指定了首选源，优先选择
	if preferredSource != "" {
		for _, candidate := range candidates {
			if candidate.Source == preferredSource {
				return &candidate
			}
		}
	}
	
	// 按源的优先级排序
	sourcePriority := map[string]int{
		"adoptium": 1,
		"corretto": 2,
		"zulu":     3,
		"oracle":   4,
		"graalvm":  5,
	}
	
	sort.Slice(candidates, func(i, j int) bool {
		pi := sourcePriority[candidates[i].Source]
		pj := sourcePriority[candidates[j].Source]
		if pi == 0 {
			pi = 999
		}
		if pj == 0 {
			pj = 999
		}
		return pi < pj
	})
	
	return &candidates[0]
}

// GetVersionSuggestions 获取版本建议
func (r *Resolver) GetVersionSuggestions(input string, releases []sources.JavaRelease) []string {
	var suggestions []string
	input = strings.ToLower(input)
	
	// 添加别名建议
	aliases := r.GetSupportedAliases()
	for _, alias := range aliases {
		if strings.HasPrefix(alias.Alias, input) {
			suggestions = append(suggestions, alias.Alias)
		}
	}
	
	// 添加版本号建议
	versionMap := make(map[string]bool)
	for _, release := range releases {
		version := fmt.Sprintf("%d", release.MajorVersion)
		if strings.HasPrefix(version, input) && !versionMap[version] {
			suggestions = append(suggestions, version)
			versionMap[version] = true
		}
		
		if strings.HasPrefix(release.FullVersion, input) && !versionMap[release.FullVersion] {
			suggestions = append(suggestions, release.FullVersion)
			versionMap[release.FullVersion] = true
		}
	}
	
	// 添加源特定建议
	sourceMap := make(map[string]bool)
	for _, release := range releases {
		sourceAlias := release.Source + "-latest"
		if strings.HasPrefix(sourceAlias, input) && !sourceMap[sourceAlias] {
			suggestions = append(suggestions, sourceAlias)
			sourceMap[sourceAlias] = true
		}
	}
	
	return suggestions
}

// ExplainAlias 解释别名的含义
func (r *Resolver) ExplainAlias(alias string, releases []sources.JavaRelease) (string, error) {
	alias = strings.ToLower(strings.TrimSpace(alias))
	
	// 查找别名定义
	aliases := r.GetSupportedAliases()
	for _, a := range aliases {
		if a.Alias == alias {
			// 尝试解析别名以获取具体版本
			if resolved, err := r.ResolveAlias(alias, releases, ""); err == nil {
				return fmt.Sprintf("%s: %s (resolves to Java %s from %s)", 
					a.Alias, a.Description, resolved.FullVersion, resolved.Vendor), nil
			}
			return fmt.Sprintf("%s: %s", a.Alias, a.Description), nil
		}
	}
	
	return "", fmt.Errorf("unknown alias: %s", alias)
}
