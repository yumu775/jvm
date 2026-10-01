package download

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"jvm/internal/alias"
	"jvm/internal/sources"
)

// Downloader 负责下载和安装 Java 版本
type Downloader struct {
	client        *http.Client
	sourceManager *sources.SourceManager
	aliasResolver *alias.Resolver
	query         func(sources.QueryOptions) (sources.CatalogResult, error)
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
	return d.GetAvailableVersionsFromSources(nil)
}
func (d *Downloader) GetAvailableVersionsFromSources(names []string) ([]sources.JavaRelease, error) {
	result, err := d.QueryCatalog(sources.QueryOptions{Sources: names})
	return result.Releases, err
}

func (d *Downloader) QueryCatalog(options sources.QueryOptions) (sources.CatalogResult, error) {
	if d.query != nil {
		return d.query(options)
	}
	result, err := sources.NewCatalog().Query(options)
	for _, report := range result.Reports {
		if report.Error != "" {
			fmt.Fprintf(os.Stderr, "Warning [%s/%s]: %s\n", report.Source, report.Status, report.Error)
		}
	}
	return result, err
}
func (d *Downloader) FindVersion(version, source string) (*sources.JavaRelease, error) {
	if strings.HasSuffix(version, "-latest") {
		named := strings.TrimSuffix(version, "-latest")
		if source != "" && source != named {
			return nil, fmt.Errorf("conflicting sources")
		}
		source = named
	}
	if source == "" {
		var err error
		source, err = d.sourceManager.DefaultSource()
		if err != nil {
			return nil, err
		}
	}
	major := 0
	parts := strings.FieldsFunc(version, func(r rune) bool { return r == '.' || r == '+' })
	if len(parts) > 0 {
		major, _ = strconv.Atoi(parts[0])
	}
	all := major > 0 && strings.ContainsAny(version, ".+")
	result, err := d.QueryCatalog(sources.QueryOptions{Sources: []string{source}, Major: major, AllVersions: all})
	if err != nil {
		return nil, err
	}
	return d.aliasResolver.ResolveAlias(version, result.Releases, source)
}
func (d *Downloader) FindVersionFromSource(version, source string) (*sources.JavaRelease, error) {
	return d.FindVersion(version, source)
}

// DownloadJava 下载指定版本的 Java
// 这个函数展示了如何下载大文件并显示进度
func (d *Downloader) DownloadJava(release *sources.JavaRelease, downloadDir string) (string, error) {
	if release == nil {
		return "", fmt.Errorf("missing release")
	}
	// 旧目录缓存也可能保存 Azul 约整后的 size；已有校验和时不会再取详情。
	// 清除非精确长度，仍由 HTTP 长度和强制 SHA-256 校验实际内容。
	if release.Metadata["metadata_provider"] == "azul" {
		release.FileSize = 0
	}
	if release.Checksum == "" {
		if err := d.sourceManager.PrepareRelease(release); err != nil {
			return "", fmt.Errorf("prepare verified download: %w", err)
		}
	}
	if release.FileName == "" || filepath.Base(release.FileName) != release.FileName || strings.ContainsAny(release.FileName, "/\\:") || release.FileName == "." || release.FileName == ".." {
		return "", fmt.Errorf("invalid archive filename")
	}
	checksum, err := hex.DecodeString(release.Checksum)
	if err != nil || len(checksum) != sha256.Size {
		return "", fmt.Errorf("valid SHA-256 checksum is required")
	}
	if err = os.MkdirAll(downloadDir, 0755); err != nil {
		return "", err
	}
	target := filepath.Join(downloadDir, release.FileName)
	if info, e := os.Lstat(target); e == nil {
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("cache path is not a regular file")
		}
		if verifyArchive(target, release) == nil {
			return target, nil
		}
		if err = os.Remove(target); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(e) {
		return "", e
	}
	resp, err := d.getWithRetry(release.DownloadURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}
	f, err := os.CreateTemp(downloadDir, ".download-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if err = d.downloadWithProgress(resp.Body, f, resp.ContentLength); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	if err = verifyArchive(f.Name(), release); err != nil {
		return "", err
	}
	if err = os.Rename(f.Name(), target); err != nil {
		return "", err
	}
	return target, nil
}

// getWithRetry 对短暂网络故障与限流重试；404和校验失败不会静默换包或厂商。
func (d *Downloader) getWithRetry(url string) (*http.Response, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		response, err := d.client.Get(url)
		if err == nil && response.StatusCode != http.StatusTooManyRequests && response.StatusCode < 500 {
			return response, nil
		}
		if err != nil {
			last = err
		} else {
			last = fmt.Errorf("download returned HTTP %d", response.StatusCode)
			response.Body.Close()
		}
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
		}
	}
	return nil, last
}
func verifyArchive(path string, release *sources.JavaRelease) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return err
	}
	if release.FileSize > 0 && n != release.FileSize {
		return fmt.Errorf("archive size mismatch: expected %d bytes, received %d bytes", release.FileSize, n)
	}
	if !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), release.Checksum) {
		return fmt.Errorf("archive SHA-256 mismatch")
	}
	return nil
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
	fmt.Fprintln(os.Stderr)
	return err
}

// ProgressWriter 实现带进度显示的 Writer
type ProgressWriter struct {
	Writer      io.Writer
	Total       int64
	Current     int64
	StartTime   time.Time
	lastPrinted time.Time
}

// Write 实现 io.Writer 接口，并显示下载进度
func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.Writer.Write(p)
	if err != nil {
		return n, err
	}

	pw.Current += int64(n)
	if time.Since(pw.lastPrinted) >= 200*time.Millisecond || (pw.Total > 0 && pw.Current >= pw.Total) {
		pw.printProgress()
		pw.lastPrinted = time.Now()
	}
	return n, nil
}

// printProgress 打印下载进度
func (pw *ProgressWriter) printProgress() {
	if pw.Total <= 0 {
		fmt.Fprintf(os.Stderr, "\rDownloaded: %s", formatBytes(pw.Current))
		return
	}

	percentage := float64(pw.Current) / float64(pw.Total) * 100
	elapsed := time.Since(pw.StartTime)

	var eta time.Duration
	if pw.Current > 0 {
		eta = time.Duration(float64(elapsed) * (float64(pw.Total)/float64(pw.Current) - 1))
	}

	// 创建进度条
	barWidth := 30
	filled := int(percentage / 100 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	fmt.Fprintf(os.Stderr, "\r[%s] %.1f%% (%s/%s) ETA: %s",
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
