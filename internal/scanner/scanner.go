package scanner

import (
	"context"
	"jvm/internal/config"
	"jvm/internal/version"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
)

// Scanner 负责扫描和识别系统中已安装的 Java 版本
type Scanner struct{}

// NewScanner 创建一个新的扫描器实例
func NewScanner() *Scanner {
	return &Scanner{}
}

// JavaInstallation 表示一个已安装的 Java 版本
type JavaInstallation struct {
	Version      string // 版本号，如 "17.0.8"
	MajorVersion int    // 主版本号，如 17
	Path         string // 安装路径
	Vendor       string // 供应商，如 "Eclipse Adoptium"
	Type         string // 类型，如 "JDK", "JRE"
	Architecture string // 架构，如 "x64"
	Source       string // 来源，如 "system", "jvm-managed"
}

// ScanSystemJava 扫描系统中已安装的 Java 版本
func (s *Scanner) ScanSystemJava() ([]JavaInstallation, error) {
	color.Blue("Scanning for existing Java installations...")

	var installations []JavaInstallation

	// 扫描常见的 Java 安装路径
	searchPaths := s.getCommonJavaPaths()
	if cfg, err := config.LoadConfig(); err == nil {
		searchPaths = append(searchPaths, cfg.CustomScanPaths...)
	}

	for _, path := range searchPaths {
		if installs, err := s.scanDirectory(path); err == nil {
			installations = append(installations, installs...)
		}
	}

	// 扫描环境变量中的 Java
	if envJava := s.scanEnvironmentJava(); envJava != nil {
		installations = append(installations, *envJava)
	}

	// 去重
	installations = s.deduplicateInstallations(installations)

	color.Green("Found %d Java installation(s)", len(installations))
	return installations, nil
}

// ScanCustomPaths 扫描指定路径中的 Java 安装
func (s *Scanner) ScanCustomPaths(paths []string) ([]JavaInstallation, error) {
	color.Blue("Scanning custom paths for Java installations...")

	var installations []JavaInstallation

	for _, path := range paths {
		color.Blue("Scanning path: %s", path)

		if installs, err := s.scanDirectory(path); err == nil {
			installations = append(installations, installs...)
		} else {
			color.Yellow("Warning: failed to scan %s: %v", path, err)
		}
	}

	// 去重
	installations = s.deduplicateInstallations(installations)

	color.Green("Found %d Java installation(s) in custom paths", len(installations))
	return installations, nil
}

// getCommonJavaPaths 获取常见的 Java 安装路径
func (s *Scanner) getCommonJavaPaths() []string {
	var paths []string

	switch runtime.GOOS {
	case "windows":
		paths = []string{
			"C:\\Program Files\\Java",
			"C:\\Program Files (x86)\\Java",
			"C:\\Program Files\\Eclipse Adoptium",
			"C:\\Program Files\\Eclipse Foundation",
			"C:\\Program Files\\Amazon Corretto",
			"C:\\Program Files\\Zulu",
			"C:\\Java",
		}

		// 动态添加常见驱动器的 Java 目录
		drives := []string{"C:", "D:", "E:", "F:"}
		commonDirs := []string{"Java", "JavaVersions", "JDK", "jdk"}

		for _, drive := range drives {
			for _, dir := range commonDirs {
				paths = append(paths, filepath.Join(drive+"\\", dir))
			}
		}

		// 添加用户目录下的 Java 安装
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			paths = append(paths, filepath.Join(userProfile, "Java"))
			paths = append(paths, filepath.Join(userProfile, ".jdks"))
		}

	case "darwin": // macOS
		paths = []string{
			"/Library/Java/JavaVirtualMachines",
			"/System/Library/Java/JavaVirtualMachines",
			"/usr/local/opt",
			"/opt/homebrew/opt",
		}

		// 添加用户目录下的 Java 安装
		if home := os.Getenv("HOME"); home != "" {
			paths = append(paths, filepath.Join(home, ".jdks"))
			paths = append(paths, filepath.Join(home, "Library/Java/JavaVirtualMachines"))
		}

	case "linux":
		paths = []string{
			"/usr/lib/jvm",
			"/usr/java",
			"/opt/java",
			"/opt/jdk",
			"/usr/local/java",
			"/usr/local/jdk",
		}

		// 添加用户目录下的 Java 安装
		if home := os.Getenv("HOME"); home != "" {
			paths = append(paths, filepath.Join(home, ".jdks"))
			paths = append(paths, filepath.Join(home, "java"))
		}
	}

	return paths
}

// scanDirectory 扫描指定目录中的 Java 安装
func (s *Scanner) scanDirectory(dir string) ([]JavaInstallation, error) {
	return s.scanDepth(dir, 0, make(map[string]bool))
}

func (s *Scanner) scanDepth(dir string, depth int, seen map[string]bool) ([]JavaInstallation, error) {
	if depth > 4 {
		return nil, nil
	}
	real, err := filepath.EvalSymlinks(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	key := real
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	if seen[key] {
		return nil, nil
	}
	seen[key] = true
	if installation := s.analyzeJavaInstallation(real); installation != nil {
		return []JavaInstallation{*installation}, nil
	}
	entries, err := os.ReadDir(real)
	if err != nil {
		return nil, err
	}
	var result []JavaInstallation
	for _, entry := range entries {
		if !s.shouldScanSubdirectory(entry.Name()) {
			continue
		}
		p := filepath.Join(real, entry.Name())
		info, e := os.Stat(p)
		if e != nil || !info.IsDir() {
			continue
		}
		items, e := s.scanDepth(p, depth+1, seen)
		if e == nil {
			result = append(result, items...)
		}
	}
	return result, nil
}

// shouldScanSubdirectory 判断是否应该扫描子目录
func (s *Scanner) shouldScanSubdirectory(name string) bool {
	// 避免扫描明显不是 Java 安装的目录
	skipDirs := []string{"bin", "lib", "include", "man", "docs", "demo", "sample"}
	for _, skip := range skipDirs {
		if strings.EqualFold(name, skip) {
			return false
		}
	}
	return true
}

// AnalyzeJavaInstallation 分析目录是否包含有效的 Java 安装（公开方法）
func (s *Scanner) AnalyzeJavaInstallation(path string) *JavaInstallation {
	return s.analyzeJavaInstallation(path)
}

// analyzeJavaInstallation 分析目录是否包含有效的 Java 安装
func (s *Scanner) analyzeJavaInstallation(path string) *JavaInstallation {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil
	}
	path = absolute
	// macOS 的 .jdk 包将 JAVA_HOME 放在 Contents/Home。
	if info, err := os.Stat(filepath.Join(path, "Contents", "Home", "bin")); err == nil && info.IsDir() {
		path = filepath.Join(path, "Contents", "Home")
	}
	// 检查是否有 bin 目录
	binDir := filepath.Join(path, "bin")
	if info, err := os.Stat(binDir); err != nil || !info.IsDir() {
		return nil
	}

	// 检查 java 可执行文件
	javaExe := "java"
	if runtime.GOOS == "windows" {
		javaExe = "java.exe"
	}

	javaPath := filepath.Join(binDir, javaExe)
	if info, err := os.Stat(javaPath); err != nil || !info.Mode().IsRegular() {
		return nil
	}

	// 获取 Java 版本信息
	version, vendor, javaType, arch := s.getJavaInfo(javaPath)
	if version == "" {
		return nil
	}

	majorVersion := s.extractMajorVersion(version)

	return &JavaInstallation{
		Version:      version,
		MajorVersion: majorVersion,
		Path:         path,
		Vendor:       vendor,
		Type:         javaType,
		Architecture: arch,
		Source:       "system",
	}
}

// getJavaInfo 获取 Java 版本信息
func (s *Scanner) getJavaInfo(javaPath string) (version, vendor, javaType, arch string) {
	// 优先读取 JDK 自带元数据，扫描无需启动正常安装的 Java。
	if data, err := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(javaPath)), "release")); err == nil {
		fields := map[string]string{}
		for _, line := range strings.Split(string(data), "\n") {
			key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
			if ok {
				fields[key] = strings.Trim(value, "\"")
			}
		}
		v := s.normalizeVersion(fields["JAVA_VERSION"])
		if s.extractMajorVersion(v) > 0 {
			vendor, arch = fields["IMPLEMENTOR"], fields["OS_ARCH"]
			if vendor == "" {
				vendor = "Unknown"
			}
			switch arch {
			case "amd64", "x86_64":
				arch = "x64"
			case "arm64":
				arch = "aarch64"
			case "i386", "x86":
				arch = "x32"
			}
			if arch == "" {
				arch = runtime.GOARCH
			}
			javaType = "JRE"
			suffix := ""
			if runtime.GOOS == "windows" {
				suffix = ".exe"
			}
			if info, err := os.Stat(filepath.Join(filepath.Dir(javaPath), "javac"+suffix)); err == nil && info.Mode().IsRegular() {
				javaType = "JDK"
			}
			return v, vendor, javaType, arch
		}
	}
	// 执行 java -version 命令
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, javaPath, "-version")
	cmd.WaitDelay = time.Second
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", "", ""
	}

	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")

	// 解析版本信息
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 解析版本号
		if version == "" {
			if versionMatch := regexp.MustCompile(`version "([^"]+)"`).FindStringSubmatch(line); len(versionMatch) > 1 {
				version = s.normalizeVersion(versionMatch[1])
			}
		}

		// 解析供应商信息
		if vendor == "" {
			if strings.Contains(line, "OpenJDK") {
				if strings.Contains(line, "Eclipse Adoptium") || strings.Contains(line, "Temurin") {
					vendor = "Eclipse Adoptium"
				} else if strings.Contains(line, "Amazon") {
					vendor = "Amazon Corretto"
				} else if strings.Contains(line, "Zulu") {
					vendor = "Azul Zulu"
				} else {
					vendor = "OpenJDK"
				}
			} else if strings.Contains(line, "Oracle") {
				vendor = "Oracle"
			}
		}

		// 解析架构信息
		if arch == "" {
			if strings.Contains(line, "64-Bit") || strings.Contains(line, "amd64") {
				arch = "x64"
			} else if strings.Contains(line, "32-Bit") || strings.Contains(line, "i386") {
				arch = "x32"
			} else if strings.Contains(line, "aarch64") || strings.Contains(line, "arm64") {
				arch = "aarch64"
			}
		}
	}

	// 确定是 JDK 还是 JRE
	javaType = "JRE"
	if _, err := os.Stat(filepath.Join(filepath.Dir(filepath.Dir(javaPath)), "bin", "javac")); err == nil {
		javaType = "JDK"
	} else if _, err := os.Stat(filepath.Join(filepath.Dir(filepath.Dir(javaPath)), "bin", "javac.exe")); err == nil {
		javaType = "JDK"
	}

	// 设置默认值
	if vendor == "" {
		vendor = "Unknown"
	}
	if arch == "" {
		arch = runtime.GOARCH
	}

	return version, vendor, javaType, arch
}

// normalizeVersion 规范化版本号
func (s *Scanner) normalizeVersion(version string) string {
	// 移除前缀（如 "1.8.0_382" -> "8.0.382"）
	if strings.HasPrefix(version, "1.8.") {
		version = "8." + version[4:]
	} else if strings.HasPrefix(version, "1.") {
		// 处理其他 1.x 版本
		parts := strings.Split(version, ".")
		if len(parts) >= 2 {
			version = parts[1] + "." + strings.Join(parts[2:], ".")
		}
	}

	// 替换下划线为点号
	version = strings.ReplaceAll(version, "_", ".")

	return version
}

// extractMajorVersion 提取主版本号
func (s *Scanner) extractMajorVersion(version string) int {
	parts := strings.Split(version, ".")
	if len(parts) > 0 {
		if major := strings.TrimSpace(parts[0]); major != "" {
			if majorInt, err := strconv.Atoi(major); err == nil {
				return majorInt
			}
		}
	}
	return 0
}

// scanEnvironmentJava 扫描环境变量中的 Java
func (s *Scanner) scanEnvironmentJava() *JavaInstallation {
	javaHome := os.Getenv("JAVA_HOME")
	if javaHome == "" {
		return nil
	}

	return s.analyzeJavaInstallation(javaHome)
}

// deduplicateInstallations 去除重复的安装
func (s *Scanner) deduplicateInstallations(installations []JavaInstallation) []JavaInstallation {
	seen := make(map[string]bool)
	var result []JavaInstallation

	for _, installation := range installations {
		key, _ := filepath.Abs(installation.Path)
		if real, err := filepath.EvalSymlinks(key); err == nil {
			key = real
		}
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if !seen[key] {
			seen[key] = true
			result = append(result, installation)
		}
	}

	return result
}

// ImportInstallation 导入已存在的 Java 安装到 jvm 管理
func (s *Scanner) ImportInstallation(installation JavaInstallation, jvmVersionsDir string) error {
	manager, err := version.NewManager()
	if err != nil {
		return err
	}
	return manager.Register(installation.Version, installation.Path, false)
}
