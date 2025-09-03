package scanner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

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
	Version     string // 版本号，如 "17.0.8"
	MajorVersion int   // 主版本号，如 17
	Path        string // 安装路径
	Vendor      string // 供应商，如 "Eclipse Adoptium"
	Type        string // 类型，如 "JDK", "JRE"
	Architecture string // 架构，如 "x64"
	Source      string // 来源，如 "system", "jvm-managed"
}

// ScanSystemJava 扫描系统中已安装的 Java 版本
func (s *Scanner) ScanSystemJava() ([]JavaInstallation, error) {
	color.Blue("Scanning for existing Java installations...")
	
	var installations []JavaInstallation
	
	// 扫描常见的 Java 安装路径
	searchPaths := s.getCommonJavaPaths()
	
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
	var installations []JavaInstallation
	
	// 检查目录是否存在
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return installations, nil
	}
	
	// 遍历目录
	entries, err := os.ReadDir(dir)
	if err != nil {
		return installations, err
	}
	
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		
		subDir := filepath.Join(dir, entry.Name())
		
		// 检查是否是有效的 Java 安装
		if installation := s.analyzeJavaInstallation(subDir); installation != nil {
			installation.Source = "system"
			installations = append(installations, *installation)
		}
		
		// 递归扫描子目录（限制深度）
		if s.shouldScanSubdirectory(entry.Name()) {
			if subInstalls, err := s.scanDirectory(subDir); err == nil {
				installations = append(installations, subInstalls...)
			}
		}
	}
	
	return installations, nil
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
	// 检查是否有 bin 目录
	binDir := filepath.Join(path, "bin")
	if _, err := os.Stat(binDir); os.IsNotExist(err) {
		return nil
	}
	
	// 检查 java 可执行文件
	javaExe := "java"
	if runtime.GOOS == "windows" {
		javaExe = "java.exe"
	}
	
	javaPath := filepath.Join(binDir, javaExe)
	if _, err := os.Stat(javaPath); os.IsNotExist(err) {
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
	// 执行 java -version 命令
	cmd := exec.Command(javaPath, "-version")
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
		key := installation.Path
		if !seen[key] {
			seen[key] = true
			result = append(result, installation)
		}
	}
	
	return result
}

// ImportInstallation 导入已存在的 Java 安装到 jvm 管理
func (s *Scanner) ImportInstallation(installation JavaInstallation, jvmVersionsDir string) error {
	// 创建符号链接或复制安装
	targetDir := filepath.Join(jvmVersionsDir, fmt.Sprintf("java-%s", installation.Version))
	
	// 检查目标目录是否已存在
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("version %s already managed by jvm", installation.Version)
	}
	
	// 创建符号链接（在支持的系统上）
	if runtime.GOOS != "windows" {
		if err := os.Symlink(installation.Path, targetDir); err == nil {
			color.Green("Imported Java %s from %s", installation.Version, installation.Path)
			return nil
		}
	}
	
	// 如果符号链接失败，提示用户手动操作
	color.Yellow("Cannot create symbolic link. Please manually copy or move the installation:")
	color.Yellow("From: %s", installation.Path)
	color.Yellow("To: %s", targetDir)
	
	return fmt.Errorf("manual import required")
}
