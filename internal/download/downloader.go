package download

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"jvm/internal/alias"
	"jvm/internal/sources"
)

// Downloader 负责下载和安装 Java 版本
type Downloader struct {
	client        *http.Client
	sourceManager *sources.SourceManager
	aliasResolver *alias.Resolver
}

// NewDownloader 创建一个新的下载器实例
func NewDownloader() *Downloader {
	return &Downloader{
		client: &http.Client{
			Timeout: 30 * time.Minute, // 30分钟超时，因为 Java 安装包比较大
		},
		sourceManager: sources.NewSourceManager(),
		aliasResolver: alias.NewResolver(),
	}
}



// GetAvailableVersions 获取可用的 Java 版本列表
func (d *Downloader) GetAvailableVersions() ([]sources.JavaRelease, error) {
	// 获取默认源配置
	javaSources := d.sourceManager.GetDefaultSources()

	// 从所有启用的源获取版本
	return d.sourceManager.GetAvailableVersions(javaSources)
}

// GetAvailableVersionsFromSources 从指定源获取版本
func (d *Downloader) GetAvailableVersionsFromSources(sourceNames []string) ([]sources.JavaRelease, error) {
	allSources := d.sourceManager.GetDefaultSources()
	var enabledSources []sources.JavaSource

	if len(sourceNames) == 0 {
		// 如果没有指定源，使用所有启用的源
		for _, source := range allSources {
			if source.Enabled {
				enabledSources = append(enabledSources, source)
			}
		}
	} else {
		// 只使用指定的源
		for _, sourceName := range sourceNames {
			for _, source := range allSources {
				if source.Name == sourceName {
					enabledSources = append(enabledSources, source)
					break
				}
			}
		}
	}

	return d.sourceManager.GetAvailableVersions(enabledSources)
}

// FindVersion 根据版本号或别名查找对应的发布信息
func (d *Downloader) FindVersion(version string, preferredSource string) (*sources.JavaRelease, error) {
	// 获取可用版本
	releases, err := d.GetAvailableVersions()
	if err != nil {
		return nil, err
	}

	// 尝试解析别名
	resolved, err := d.aliasResolver.ResolveAlias(version, releases, preferredSource)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve version %s: %w", version, err)
	}

	return resolved, nil
}

// FindVersionFromSource 从特定源查找版本
func (d *Downloader) FindVersionFromSource(version string, sourceName string) (*sources.JavaRelease, error) {
	releases, err := d.GetAvailableVersionsFromSources([]string{sourceName})
	if err != nil {
		return nil, err
	}

	return d.aliasResolver.ResolveAlias(version, releases, sourceName)
}

// DownloadJava 下载指定版本的 Java
// 这个函数展示了如何下载大文件并显示进度
func (d *Downloader) DownloadJava(release *sources.JavaRelease, downloadDir string) (string, error) {
	// 确保下载目录存在
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create download directory: %w", err)
	}
	
	filePath := filepath.Join(downloadDir, release.FileName)
	
	// 检查文件是否已存在
	if _, err := os.Stat(filePath); err == nil {
		color.Yellow("File already exists: %s", filePath)
		return filePath, nil
	}
	
	color.Blue("Downloading Java %s...", release.Version)
	color.Blue("URL: %s", release.DownloadURL)
	
	// 创建 HTTP 请求
	resp, err := d.client.Get(release.DownloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download Java: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}
	
	// 创建目标文件
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	
	// 获取文件大小用于显示进度
	fileSize := resp.ContentLength
	if fileSize <= 0 {
		fileSize = release.FileSize
	}
	
	// 下载文件并显示进度
	if err := d.downloadWithProgress(resp.Body, file, fileSize); err != nil {
		os.Remove(filePath) // 下载失败时清理文件
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	
	color.Green("Download completed: %s", filePath)
	return filePath, nil
}

// downloadWithProgress 下载文件并显示进度
func (d *Downloader) downloadWithProgress(src io.Reader, dst io.Writer, totalSize int64) error {
	// 创建一个带进度显示的 Writer
	progressWriter := &ProgressWriter{
		Writer:    dst,
		Total:     totalSize,
		Current:   0,
		StartTime: time.Now(),
	}
	
	_, err := io.Copy(progressWriter, src)
	fmt.Println() // 换行
	return err
}



// ProgressWriter 实现带进度显示的 Writer
type ProgressWriter struct {
	Writer    io.Writer
	Total     int64
	Current   int64
	StartTime time.Time
}

// Write 实现 io.Writer 接口，并显示下载进度
func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.Writer.Write(p)
	if err != nil {
		return n, err
	}
	
	pw.Current += int64(n)
	pw.printProgress()
	return n, nil
}

// printProgress 打印下载进度
func (pw *ProgressWriter) printProgress() {
	if pw.Total <= 0 {
		fmt.Printf("\rDownloaded: %s", formatBytes(pw.Current))
		return
	}
	
	percentage := float64(pw.Current) / float64(pw.Total) * 100
	elapsed := time.Since(pw.StartTime)
	
	var eta time.Duration
	if pw.Current > 0 {
		eta = time.Duration(float64(elapsed) * (float64(pw.Total) / float64(pw.Current) - 1))
	}
	
	// 创建进度条
	barWidth := 30
	filled := int(percentage / 100 * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	
	fmt.Printf("\r[%s] %.1f%% (%s/%s) ETA: %s",
		bar,
		percentage,
		formatBytes(pw.Current),
		formatBytes(pw.Total),
		eta.Round(time.Second))
}

// formatBytes 格式化字节数为人类可读的格式
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
