//go:build windows

package cmd

import (
	"errors"
	"fmt"
	"runtime"
	"time"

	"golang.org/x/sys/windows"
)

func environmentCommandMutexName() (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("cannot identify user for environment lock: %w", err)
	}
	// 不依赖 JVM_HOME，同一用户的不同仓库共享同一组 Windows 环境。
	return `Global\JVM.Environment.` + user.User.Sid.String(), nil
}

func acquireEnvironmentCommandLock() (func() error, error) {
	name, err := environmentCommandMutexName()
	if err != nil {
		return nil, err
	}
	return acquireNamedEnvironmentLock(name, 15*time.Second)
}

func acquireNamedEnvironmentLock(name string, timeout time.Duration) (func() error, error) {
	encoded, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateMutexEx(nil, encoded, 0, windows.SYNCHRONIZE|windows.MUTEX_MODIFY_STATE)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return nil, fmt.Errorf("cannot open environment command lock: %w", err)
	}
	// Windows mutex 由线程拥有，Go 协程不能在等待与释放之间迁移线程。
	runtime.LockOSThread()
	status, err := windows.WaitForSingleObject(handle, uint32(timeout/time.Millisecond))
	if err != nil || (status != windows.WAIT_OBJECT_0 && status != windows.WAIT_ABANDONED) {
		windows.CloseHandle(handle)
		runtime.UnlockOSThread()
		if status == uint32(windows.WAIT_TIMEOUT) {
			return nil, fmt.Errorf("another JVM command is changing this user's environment; lock wait timed out after %s, retry when it finishes", timeout)
		}
		return nil, fmt.Errorf("cannot acquire environment command lock (status %d): %v", status, err)
	}
	return func() error {
		defer runtime.UnlockOSThread()
		return errors.Join(windows.ReleaseMutex(handle), windows.CloseHandle(handle))
	}, nil
}
