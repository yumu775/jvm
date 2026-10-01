package install

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

// Installer 负责安装和解压 Java 包
type Installer struct{}

// NewInstaller 创建一个新的安装器实例
func NewInstaller() *Installer {
	return &Installer{}
}

// InstallJava 安装 Java 到指定目录
func (i *Installer) InstallJava(archivePath, installDir, version string) error {
	color.Blue("Installing Java %s to %s...", version, installDir)

	// 确保安装目录存在
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// 根据文件扩展名选择解压方法
	if strings.HasSuffix(archivePath, ".zip") {
		return i.extractZip(archivePath, installDir)
	} else if strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
		return i.extractTarGz(archivePath, installDir)
	} else {
		return fmt.Errorf("unsupported archive format: %s", archivePath)
	}
}

// extractZip 解压 ZIP 文件
// 这个函数展示了如何处理 ZIP 压缩包
func (i *Installer) extractZip(src, dest string) error {
	color.Blue("Extracting ZIP archive...")

	// 打开 ZIP 文件
	reader, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	// 提取文件
	for _, file := range reader.File {
		if err := i.extractZipFile(file, dest); err != nil {
			return fmt.Errorf("failed to extract file %s: %w", file.Name, err)
		}
	}

	color.Green("ZIP extraction completed")
	return nil
}

// extractZipFile 提取单个 ZIP 文件
func (i *Installer) extractZipFile(file *zip.File, dest string) error {
	// 构建目标路径
	path, err := safeArchivePath(dest, file.Name)
	if err != nil {
		return err
	}

	// 如果是目录，创建目录
	if file.FileInfo().IsDir() {
		return os.MkdirAll(path, file.FileInfo().Mode())
	}

	// 确保父目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// 打开源文件
	fileReader, err := file.Open()
	if err != nil {
		return err
	}
	defer fileReader.Close()

	// 创建目标文件
	targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.FileInfo().Mode())
	if err != nil {
		return err
	}
	defer targetFile.Close()

	// 复制文件内容
	_, err = io.Copy(targetFile, fileReader)
	return err
}

// extractTarGz 解压 tar.gz 文件
// 这个函数展示了如何处理 tar.gz 压缩包
func (i *Installer) extractTarGz(src, dest string) error {
	color.Blue("Extracting tar.gz archive...")

	// 打开文件
	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open tar.gz file: %w", err)
	}
	defer file.Close()

	// 创建 gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	// 创建 tar reader
	tarReader := tar.NewReader(gzipReader)

	// 提取文件
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // 到达文件末尾
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		if err := i.extractTarFile(tarReader, header, dest); err != nil {
			return fmt.Errorf("failed to extract file %s: %w", header.Name, err)
		}
	}

	color.Green("tar.gz extraction completed")
	return nil
}

// extractTarFile 提取单个 tar 文件
func (i *Installer) extractTarFile(tarReader *tar.Reader, header *tar.Header, dest string) error {
	// 构建目标路径
	path, err := safeArchivePath(dest, header.Name)
	if err != nil {
		return err
	}

	// 根据文件类型处理
	switch header.Typeflag {
	case tar.TypeDir:
		// 创建目录
		return os.MkdirAll(path, os.FileMode(header.Mode))

	case tar.TypeReg:
		// 创建普通文件
		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		// 创建文件
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
		if err != nil {
			return err
		}
		defer file.Close()

		// 复制文件内容
		_, err = io.Copy(file, tarReader)
		return err

	case tar.TypeSymlink:
		// 创建符号链接
		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		if filepath.IsAbs(header.Linkname) || strings.Contains(header.Linkname, ":") {
			return fmt.Errorf("absolute link target rejected")
		}
		target := filepath.Join(filepath.Dir(path), filepath.FromSlash(header.Linkname))
		relative, err := filepath.Rel(dest, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("link escapes installation")
		}
		return os.Symlink(header.Linkname, path)

	default:
		// 忽略其他类型的文件
		return nil
	}
}

// NormalizeInstallation 规范化安装目录结构
// 有些 Java 发行版会在压缩包中包含额外的目录层级
func (i *Installer) NormalizeInstallation(installDir string) error {
	if hasJDK(installDir) {
		return validateInstallationLinks(installDir)
	}
	entries, err := os.ReadDir(installDir)
	if err != nil {
		return err
	}
	var candidate string
	if hasJDK(filepath.Join(installDir, "Contents", "Home")) {
		candidate = filepath.Join(installDir, "Contents", "Home")
	} else if len(entries) == 1 && entries[0].IsDir() {
		sub := filepath.Join(installDir, entries[0].Name())
		if hasJDK(sub) {
			candidate = sub
		} else if hasJDK(filepath.Join(sub, "Contents", "Home")) {
			candidate = filepath.Join(sub, "Contents", "Home")
		}
	}
	if candidate == "" {
		return fmt.Errorf("archive does not contain a supported JDK layout")
	}
	if err := i.moveDirectoryContents(candidate, installDir); err != nil {
		return err
	}
	return validateInstallationLinks(installDir)
}
func hasJDK(path string) bool {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	for _, name := range []string{"java", "javac"} {
		info, err := os.Stat(filepath.Join(path, "bin", name+suffix))
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

// moveDirectoryContents 移动目录中的所有内容到另一个目录
func (i *Installer) moveDirectoryContents(src, dest string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		if err := os.Rename(srcPath, destPath); err != nil {
			return fmt.Errorf("failed to move %s to %s: %w", srcPath, destPath, err)
		}
	}

	return nil
}

// ValidateInstallation 验证 Java 安装是否有效
func (i *Installer) ValidateInstallation(installDir string) error {
	if !hasJDK(installDir) {
		return fmt.Errorf("installation must contain java and javac executables")
	}
	for _, name := range []string{"bin", "lib"} {
		info, err := os.Stat(filepath.Join(installDir, name))
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", name)
		}
	}
	return validateInstallationLinks(installDir)
}

// safeArchivePath 验证目录边界，并拒绝通过已有符号链接写入。
func safeArchivePath(root, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) || strings.Contains(name, ":") {
		return "", fmt.Errorf("invalid archive path: %s", name)
	}
	path := filepath.Join(root, filepath.FromSlash(name))
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive path escapes installation: %s", name)
	}
	current := root
	if info, err := os.Lstat(root); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("installation root is a symbolic link")
	}
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		current = filepath.Join(current, part)
		info, e := os.Lstat(current)
		if e == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("archive path traverses a symbolic link")
		}
		if e != nil && !os.IsNotExist(e) {
			return "", e
		}
	}
	return path, nil
}

// validateInstallationLinks 在目录上移后重新验证链接，避免相对目标随层级变化越界。
func validateInstallationLinks(root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	return filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if err = resolveInstallationLink(root, rel); err != nil {
			return fmt.Errorf("unsafe installation link %s: %w", path, err)
		}
		return nil
	})
}

// resolveInstallationLink 逐段解析而非先 Clean，保留链接展开后 .. 的真实语义。
// 对不存在的目标也检查边界，因此 dangling 链接不能逃出安装根。
func resolveInstallationLink(root, relative string) error {
	pending := strings.Split(filepath.ToSlash(relative), "/")
	current := root
	links := 0
	for len(pending) > 0 {
		part := pending[0]
		pending = pending[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if current == root {
				return fmt.Errorf("link target escapes installation")
			}
			current = filepath.Dir(current)
			continue
		}
		next := filepath.Join(current, part)
		info, err := os.Lstat(next)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			links++
			if links > 128 {
				return fmt.Errorf("symbolic link cycle or excessive depth")
			}
			target, err := os.Readlink(next)
			if err != nil {
				return err
			}
			if filepath.IsAbs(target) || filepath.VolumeName(target) != "" || strings.HasPrefix(filepath.ToSlash(target), "/") {
				return fmt.Errorf("absolute link target cannot be relocated safely")
			}
			pending = append(strings.Split(filepath.ToSlash(target), "/"), pending...)
			continue
		}
		current = next
	}
	return nil
}

// ValidateRelease 校验归档内 release 文件声明的 Java 主版本。
func (i *Installer) ValidateRelease(dir, expected string) error {
	if err := i.ValidateInstallation(dir); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(dir, "release"))
	if err != nil {
		return fmt.Errorf("missing JDK release metadata: %w", err)
	}
	actual := ""
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && key == "JAVA_VERSION" {
			actual = strings.Trim(value, "\"")
		}
	}
	major := func(value string) int {
		value = strings.TrimPrefix(value, "1.")
		parts := strings.FieldsFunc(value, func(r rune) bool { return r == '.' || r == '+' || r == '_' || r == '-' })
		if len(parts) == 0 {
			return 0
		}
		n, _ := strconv.Atoi(parts[0])
		return n
	}
	if major(actual) == 0 || major(expected) == 0 || major(actual) != major(expected) {
		return fmt.Errorf("release version mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}
