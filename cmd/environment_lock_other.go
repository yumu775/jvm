//go:build !windows

package cmd

// Unix 当前仍使用各配置文件已有保护；不承诺跨命令环境事务互斥。
func acquireEnvironmentCommandLock() (func() error, error) {
	return func() error { return nil }, nil
}
