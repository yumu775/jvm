package install

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	path := filepath.Join(dest, file.Name)
	
	// 安全检查：防止路径遍历攻击
	if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file path: %s", file.Name)
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
	path := filepath.Join(dest, header.Name)
	
	// 安全检查：防止路径遍历攻击
	if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file path: %s", header.Name)
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
		
		return os.Symlink(header.Linkname, path)
		
	default:
		// 忽略其他类型的文件
		return nil
	}
}

// NormalizeInstallation 规范化安装目录结构
// 有些 Java 发行版会在压缩包中包含额外的目录层级
func (i *Installer) NormalizeInstallation(installDir string) error {
	color.Blue("Normalizing installation structure...")
	
	// 检查是否有嵌套的 JDK 目录
	entries, err := os.ReadDir(installDir)
	if err != nil {
		return fmt.Errorf("failed to read install directory: %w", err)
	}
	
	// 如果只有一个目录，且看起来像 JDK 目录，则将其内容移到上级目录
	if len(entries) == 1 && entries[0].IsDir() {
		subDir := filepath.Join(installDir, entries[0].Name())
		
		// 检查是否是 JDK 目录（包含 bin 目录）
		binDir := filepath.Join(subDir, "bin")
		if _, err := os.Stat(binDir); err == nil {
			// 移动所有内容到父目录
			if err := i.moveDirectoryContents(subDir, installDir); err != nil {
				return fmt.Errorf("failed to normalize directory structure: %w", err)
			}
			
			// 删除空的子目录
			if err := os.Remove(subDir); err != nil {
				color.Yellow("Warning: failed to remove empty directory %s: %v", subDir, err)
			}
		}
	}
	
	color.Green("Installation structure normalized")
	return nil
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
	color.Blue("Validating Java installation...")
	
	// 检查必要的目录和文件
	requiredPaths := []string{
		filepath.Join(installDir, "bin"),
		filepath.Join(installDir, "lib"),
	}
	
	for _, path := range requiredPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("required path not found: %s", path)
		}
	}
	
	// 检查 java 可执行文件
	javaExe := "java"
	if filepath.Separator == '\\' { // Windows
		javaExe = "java.exe"
	}
	
	javaPath := filepath.Join(installDir, "bin", javaExe)
	if _, err := os.Stat(javaPath); os.IsNotExist(err) {
		return fmt.Errorf("java executable not found: %s", javaPath)
	}
	
	color.Green("Java installation validated successfully")
	return nil
}
