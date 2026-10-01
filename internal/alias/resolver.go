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
func (r *Resolver) ResolveAlias(input string, releases []sources.JavaRelease, source string) (*sources.JavaRelease, error) {
	input = strings.ToLower(strings.TrimSpace(input))
	if strings.HasSuffix(input, "-latest") {
		named := strings.TrimSuffix(input, "-latest")
		if source != "" && source != named {
			return nil, fmt.Errorf("conflicting sources")
		}
		source = named
		input = "latest"
	}
	var candidates []sources.JavaRelease
	for _, v := range releases {
		if source == "" || v.Source == source {
			candidates = append(candidates, v)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		c := sources.CompareVersions(candidates[i].FullVersion, candidates[j].FullVersion)
		if c != 0 {
			return c > 0
		}
		return candidates[i].Source < candidates[j].Source
	})
	if input == "stable" {
		input = "latest"
	}
	if input == "current" {
		input = "lts"
	}
	if input == "latest" {
		if len(candidates) > 0 {
			return &candidates[0], nil
		}
	}
	if input == "lts" || strings.HasPrefix(input, "lts-") {
		offset := 0
		if input != "lts" {
			var err error
			offset, err = strconv.Atoi(strings.TrimPrefix(input, "lts-"))
			if err != nil || offset < 0 {
				return nil, fmt.Errorf("invalid LTS offset: %s", input)
			}
		}
		seen := map[int]bool{}
		for _, v := range candidates {
			if !v.LTS || seen[v.MajorVersion] {
				continue
			}
			seen[v.MajorVersion] = true
			if offset == 0 {
				return &v, nil
			}
			offset--
		}
	} else {
		major, majorErr := strconv.Atoi(input)
		for _, v := range candidates {
			if v.Version == input || v.FullVersion == input || strings.Split(v.FullVersion, "+")[0] == input || (majorErr == nil && v.MajorVersion == major) {
				return &v, nil
			}
		}
	}
	return nil, fmt.Errorf("version or alias %s not available from selected source", input)
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
