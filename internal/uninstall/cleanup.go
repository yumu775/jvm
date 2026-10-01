package uninstall

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"jvm/internal/config"
	"jvm/internal/version"
)

// CleanupResult 只描述配置引用的清理，不代表删除安装文件或下载缓存。
type CleanupResult struct {
	RemovedRegistrations []string
	ClearedReferences    []string
	Retained             []string
}

// CleanupReferences 在配置事务内重新检查路径；预览不写入任何配置。
func CleanupReferences(dryRun bool) (CleanupResult, error) {
	if dryRun {
		cfg, err := config.LoadConfig()
		if err != nil {
			return CleanupResult{}, err
		}
		return cleanReferences(cfg, os.Stat, os.ReadDir), nil
	}
	var result CleanupResult
	err := config.Update(func(cfg *config.Config) error { result = cleanReferences(cfg, os.Stat, os.ReadDir); return nil })
	return result, err
}

func cleanReferences(cfg *config.Config, stat func(string) (os.FileInfo, error), readDir func(string) ([]os.DirEntry, error)) CleanupResult {
	result := CleanupResult{}
	ids := make([]string, 0, len(cfg.Installations))
	for id := range cfg.Installations {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		record := cfg.Installations[id]
		missing, reason := confirmedMissing(record.Path, stat)
		if missing {
			delete(cfg.Installations, id)
			result.RemovedRegistrations = append(result.RemovedRegistrations, id+" -> "+record.Path)
		} else if reason != "" {
			result.Retained = append(result.Retained, id+": "+reason)
		}
	}
	for _, reference := range []struct {
		name  string
		value *string
	}{{"current-version", &cfg.CurrentVersion}, {"default-version", &cfg.DefaultVersion}} {
		id := *reference.value
		if id == "" {
			continue
		}
		if version.ValidateVersion(id) != nil || id == "default" {
			// 非法旧引用不解释为路径；保留供用户显式修正，避免猜测。
			result.Retained = append(result.Retained, reference.name+": invalid identifier; select a valid installed version explicitly")
			continue
		}
		paths := []string{filepath.Join(cfg.InstallDir, "java-"+id), filepath.Join(cfg.InstallDir, id)}
		legacy, readErr := readDir(cfg.InstallDir)
		if readErr != nil && !os.IsNotExist(readErr) {
			result.Retained = append(result.Retained, reference.name+": cannot inspect legacy installation directory: "+readErr.Error())
			continue
		}
		for _, entry := range legacy {
			name := strings.TrimPrefix(entry.Name(), "java-")
			if name == id || strings.HasPrefix(name, id+".") || strings.HasPrefix(name, id+"+") {
				paths = append(paths, filepath.Join(cfg.InstallDir, entry.Name()))
			}
		}
		for registered, record := range cfg.Installations {
			if registered == id || strings.HasPrefix(registered, id+".") || strings.HasPrefix(registered, id+"+") {
				paths = append(paths, record.Path)
			}
		}
		missing := true
		for _, path := range paths {
			absent, reason := confirmedMissing(path, stat)
			if !absent {
				missing = false
				if reason != "" {
					result.Retained = append(result.Retained, reference.name+": "+reason)
				}
				break
			}
		}
		if missing {
			result.ClearedReferences = append(result.ClearedReferences, reference.name+" = "+id)
			*reference.value = ""
		}
	}
	sort.Strings(result.Retained)
	return result
}

func confirmedMissing(path string, stat func(string) (os.FileInfo, error)) (bool, string) {
	if path == "" || !filepath.IsAbs(path) {
		return false, "path is not an absolute installation directory"
	}
	_, err := stat(path)
	if err == nil {
		return false, ""
	}
	if !os.IsNotExist(err) {
		return false, fmt.Sprintf("cannot inspect %s: %v", path, err)
	}
	// 安装器替换目标期间可能短暂不存在目录，不清理其登记。
	_, lockErr := stat(path + ".install-lock")
	if lockErr == nil {
		return false, "installation operation in progress: " + path
	}
	if !os.IsNotExist(lockErr) {
		return false, fmt.Sprintf("cannot inspect installation lock %s: %v", path, lockErr)
	}
	return true, ""
}
