package env

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

// Manager 管理持久环境；当前终端必须显式执行激活脚本。
type Manager struct{}

func NewManager() *Manager { return &Manager{} }

type ShellInfo struct{ Name, ConfigFile, SetCommand, ExportCmd string }

func (m *Manager) DetectShell() (*ShellInfo, error) {
	if runtime.GOOS == "windows" {
		return &ShellInfo{Name: "windows (specify --shell for activation)"}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	name := filepath.Base(os.Getenv("SHELL"))
	if name == "." || name == "" {
		name = "bash"
	}
	config := filepath.Join(home, ".profile")
	switch name {
	case "bash":
		config = filepath.Join(home, ".bashrc")
	case "zsh":
		config = filepath.Join(home, ".zshrc")
	case "fish":
		config = filepath.Join(home, ".config", "fish", "config.fish")
	}
	return &ShellInfo{Name: name, ConfigFile: config}, nil
}

func ValidateJavaHome(home string) (string, error) {
	absolute, err := filepath.Abs(home)
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(absolute, "\r\n\x00") {
		return "", fmt.Errorf("Java path contains control characters")
	}
	if err := validatePathEntry(absolute, runtime.GOOS == "windows"); err != nil {
		return "", err
	}
	name := "java"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	info, err := os.Stat(filepath.Join(absolute, "bin", name))
	if err != nil {
		return "", fmt.Errorf("invalid Java home %q: %w", absolute, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("Java executable is a directory")
	}
	return absolute, nil
}

// 路径列表无法转义分隔符，必须在写入环境前拒绝这类目录。
func validatePathEntry(path string, windows bool) error {
	separator := ":"
	if windows {
		separator = ";"
	}
	if strings.ContainsAny(path, separator+"\r\n\x00") {
		return fmt.Errorf("path cannot be represented as a PATH entry: %q", path)
	}
	return nil
}

// UpdatePath 只替换明确管理的目录，不按 java 字样删除其他工具。
func UpdatePath(value, add, old string, windows bool) string {
	sep := ":"
	if windows {
		sep = ";"
	}
	equal := func(a, b string) bool {
		a = strings.TrimRight(strings.Trim(a, " \""), "/\\")
		b = strings.TrimRight(strings.Trim(b, " \""), "/\\")
		if windows {
			return strings.EqualFold(strings.ReplaceAll(a, "/", "\\"), strings.ReplaceAll(b, "/", "\\"))
		}
		return a == b
	}
	parts := []string{}
	if add != "" {
		parts = append(parts, add)
	}
	for _, p := range strings.Split(value, sep) {
		if p == "" {
			continue
		}
		if (add != "" && equal(p, add)) || (old != "" && equal(p, old)) {
			continue
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, sep)
}

func quotePS(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func quoteSH(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func quoteFish(s string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "'", "\\'") + "'"
}

// PowerShellIntegration 只定义函数；用户明确执行输出后才启用自动激活。
func PowerShellIntegration(executable string) string {
	program := quotePS(executable)
	return "function global:jvm {\n" +
		"  $jvmArguments = @($args)\n" +
		"  & " + program + " @jvmArguments\n" +
		"  $jvmStatus = $LASTEXITCODE\n" +
		"  $jvmTemporary = @($jvmArguments | Where-Object { $_ -match '^(--temp|-t)(=true)?$' }).Count -gt 0\n" +
		"  $jvmApply = $jvmArguments.Count -gt 0 -and ($jvmArguments[0] -eq 'use' -or $jvmArguments[0] -eq 'set-env' -or ($jvmArguments.Count -gt 1 -and $jvmArguments[0] -eq 'project' -and $jvmArguments[1] -eq 'use'))\n" +
		"  if ($jvmStatus -eq 0 -and $jvmApply -and -not $jvmTemporary -and $jvmArguments -notcontains '--help' -and $jvmArguments -notcontains '-h' -and $jvmArguments -notcontains 'list') {\n" +
		"    $jvmScript = & " + program + " env --shell powershell | Out-String\n" +
		"    $jvmStatus = $LASTEXITCODE\n" +
		"    if ($jvmStatus -eq 0) { Invoke-Expression $jvmScript }\n" +
		"  }\n" +
		"  $global:LASTEXITCODE = $jvmStatus\n" +
		"}\n"
}

// ActivationScript 返回纯脚本，不写入配置，也不修改当前进程环境。
func ActivationScript(shell, home, path, oldHome string) (string, error) {
	if strings.ContainsAny(home+path+oldHome, "\r\n\x00") {
		return "", fmt.Errorf("environment contains unsupported control characters")
	}
	windows := shell == "cmd" || shell == "powershell"
	if err := validatePathEntry(home, windows); err != nil {
		return "", err
	}
	old := ""
	if oldHome != "" {
		old = filepath.Join(oldHome, "bin")
	}
	if windows {
		path = UpdatePath(path, "", `%JAVA_HOME%\bin`, true)
	}
	updated := UpdatePath(path, filepath.Join(home, "bin"), old, windows)
	switch shell {
	case "powershell":
		return "$env:JAVA_HOME = " + quotePS(home) + "\n$env:PATH = " + quotePS(updated) + "\n", nil
	case "cmd":
		// 批处理百分号/延迟展开会再次解释路径，拒绝无法安全表达的值。
		if strings.ContainsAny(home+updated, "%!\"") {
			return "", fmt.Errorf("CMD activation cannot safely represent percent, exclamation mark or quote; use PowerShell")
		}
		return "@set \"JAVA_HOME=" + home + "\"\r\n@set \"PATH=" + updated + "\"\r\n", nil
	case "bash", "zsh", "sh":
		return "export JAVA_HOME=" + quoteSH(home) + "\nexport PATH=" + quoteSH(updated) + "\n", nil
	case "fish":
		parts := strings.Split(updated, ":")
		for i := range parts {
			parts[i] = quoteFish(parts[i])
		}
		return "set -gx JAVA_HOME " + quoteFish(home) + "\nset -gx PATH " + strings.Join(parts, " ") + "\n", nil
	default:
		return "", fmt.Errorf("unsupported shell %q; choose powershell, cmd, bash, zsh, sh or fish", shell)
	}
}

func (m *Manager) SetJavaEnvironment(home string, temporary bool) error {
	if temporary {
		return fmt.Errorf("a child process cannot change its parent shell; execute jvm env --shell <shell> --java-home <path> output in your shell")
	}
	home, err := ValidateJavaHome(home)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return persistWindowsJava(home)
	}
	shell, err := m.DetectShell()
	if err != nil {
		return err
	}
	var body string
	if shell.Name == "fish" {
		body = "set -gx JAVA_HOME " + quoteFish(home) + "\nset -gx PATH " + quoteFish(filepath.Join(home, "bin")) + " $PATH"
	} else {
		body = "export JAVA_HOME=" + quoteSH(home) + "\nexport PATH=" + quoteSH(filepath.Join(home, "bin")) + ":\"$PATH\""
	}
	return WriteProfileBlock(shell.ConfigFile, "JVM Java Version Manager", body)
}

// WriteProfileBlock 保留其他配置，遇到无法读取或损坏的标记时拒绝覆盖。
func WriteProfileBlock(path, marker, body string) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) && body == "" {
		return nil
	}
	result, err := renderProfileBlock(content, marker, body)
	if err != nil {
		return fmt.Errorf("profile %s: %w", path, err)
	}
	return writeProfileContent(path, result)
}

// ValidateProfileBlock 只读检查编码与已有标记，供安装操作在产生副作用前使用。
func ValidateProfileBlock(path, marker string) error {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
			info, err := os.Stat(parent)
			if err == nil {
				if !info.IsDir() {
					return fmt.Errorf("profile parent is not a directory: %s", parent)
				}
				return nil
			}
			if !os.IsNotExist(err) {
				return err
			}
			if filepath.Dir(parent) == parent {
				return err
			}
		}
	}
	if err != nil {
		return err
	}
	_, err = renderProfileBlock(content, marker, "")
	if err != nil {
		return fmt.Errorf("profile %s: %w", path, err)
	}
	return nil
}

func renderProfileBlock(content []byte, marker, body string) ([]byte, error) {
	bom := []byte{0xef, 0xbb, 0xbf}
	hadBOM := bytes.HasPrefix(content, bom)
	content = bytes.TrimPrefix(content, bom)
	if !utf8.Valid(content) || bytes.ContainsRune(content, 0) {
		return nil, fmt.Errorf("unsupported profile encoding; convert it to UTF-8 before continuing")
	}
	start := "# " + marker + " - START"
	end := "# " + marker + " - END"
	lines := []string{}
	inside := false
	for _, line := range strings.Split(string(content), "\n") {
		t := strings.TrimSpace(line)
		if t == start {
			if inside {
				return nil, fmt.Errorf("nested JVM block")
			}
			inside = true
			continue
		}
		if t == end {
			if !inside {
				return nil, fmt.Errorf("unmatched JVM block")
			}
			inside = false
			continue
		}
		if !inside {
			lines = append(lines, line)
		}
	}
	if inside {
		return nil, fmt.Errorf("unfinished JVM block")
	}
	result := strings.TrimRight(strings.Join(lines, "\n"), "\r\n")
	if body != "" {
		result += "\n\n" + start + "\n" + body + "\n" + end
	}
	output := []byte(result + "\n")
	// Windows PowerShell 5 需要 BOM 才能可靠读取包含中文路径的 UTF-8 配置。
	if hadBOM || (runtime.GOOS == "windows" && len(output) != utf8.RuneCount(output)) {
		output = append(bom, output...)
	}
	return output, nil
}

func writeProfileContent(path string, result []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	mode := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".jvm-profile-*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if _, err = temporary.Write(result); err != nil {
		temporary.Close()
		return err
	}
	if err = temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if err = temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), path)
}

func (m *Manager) GetCurrentJavaInfo() (map[string]string, error) {
	info := map[string]string{}
	if home := os.Getenv("JAVA_HOME"); home != "" {
		info["JAVA_HOME"] = home
	}
	if path, err := exec.LookPath("java"); err == nil {
		info["JAVA_BIN_PATH"] = filepath.Dir(path)
		info["JAVA_EXECUTABLE"] = path
	}
	return info, nil
}

// ConfigureToolPath 配置 jvm 可执行文件入口，不影响 Java 选择。
func (m *Manager) ConfigureToolPath(dir string, remove bool) error {
	if err := validatePathEntry(dir, runtime.GOOS == "windows"); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return persistWindowsToolPath(dir, remove)
	}
	shell, err := m.DetectShell()
	if err != nil {
		return err
	}
	body := ""
	if !remove {
		if shell.Name == "fish" {
			body = "fish_add_path " + quoteFish(dir)
		} else {
			body = "export PATH=" + quoteSH(dir) + ":\"$PATH\""
		}
	}
	return WriteProfileBlock(shell.ConfigFile, "JVM Tool PATH Configuration", body)
}

// ClearJavaEnvironment 仅清理正在持久使用的目标，不影响其他安装。
func (m *Manager) ClearJavaEnvironment(home string) error {
	if runtime.GOOS == "windows" {
		return clearWindowsJava(home)
	}
	shell, err := m.DetectShell()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(shell.ConfigFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	text := string(data)
	start := strings.Index(text, "# JVM Java Version Manager - START")
	end := strings.Index(text, "# JVM Java Version Manager - END")
	if start < 0 {
		return nil
	}
	if end < start {
		return fmt.Errorf("unfinished JVM block in %s", shell.ConfigFile)
	}
	block := text[start:end]
	if !strings.Contains(block, "JAVA_HOME="+quoteSH(home)+"\n") && !strings.Contains(block, "JAVA_HOME "+quoteFish(home)+"\n") {
		return nil
	}
	return WriteProfileBlock(shell.ConfigFile, "JVM Java Version Manager", "")
}
