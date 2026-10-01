//go:build windows

package env

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

type registryStore struct{ key registry.Key }

// ReadPersistentEnvironment 只读查询用户环境，不创建注册表键。
func ReadPersistentEnvironment() (map[string]string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE)
	if err == registry.ErrNotExist {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer key.Close()
	result := map[string]string{}
	for _, name := range []string{"JAVA_HOME", "Path"} {
		value, _, err := key.GetStringValue(name)
		if err == registry.ErrNotExist {
			continue
		}
		if err != nil {
			return nil, err
		}
		result[name] = value
	}
	return result, nil
}

func (s registryStore) Read(name string) (string, uint32, bool, error) {
	v, k, e := s.key.GetStringValue(name)
	if e == registry.ErrNotExist {
		return "", 0, false, nil
	}
	return v, k, e == nil, e
}
func (s registryStore) Write(name, value string, kind uint32) error {
	if kind == registry.EXPAND_SZ {
		return s.key.SetExpandStringValue(name, value)
	}
	return s.key.SetStringValue(name, value)
}
func (s registryStore) Delete(name string) error {
	err := s.key.DeleteValue(name)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}
func openEnvironment() (registryStore, error) {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	return registryStore{key}, err
}

func persistWindowsJava(home string) error {
	store, err := openEnvironment()
	if err != nil {
		return err
	}
	defer store.key.Close()
	_, kind, exists, err := store.Read("Path")
	if err != nil {
		return err
	}
	if (!exists || kind == registry.EXPAND_SZ) && strings.Contains(home, "%") {
		return fmt.Errorf("Java path containing %% cannot be stored in an expandable Windows PATH; choose a directory without %%")
	}
	// 旧版启动脚本会覆盖新的注册表选择，先移除两种 PowerShell 的旧标记块。
	if err := removeLegacyProfiles("JVM Java Version Manager"); err != nil {
		return err
	}
	if err := persistJava(store, home); err != nil {
		return err
	}
	return notifyEnvironment()
}

func removeLegacyProfiles(marker string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	for _, name := range []string{"PowerShell", "WindowsPowerShell"} {
		path := filepath.Join(home, "Documents", name, "Microsoft.PowerShell_profile.ps1")
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "# "+marker+" - START") {
			if err := WriteProfileBlock(path, marker, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

func persistWindowsToolPath(dir string, remove bool) error {
	store, err := openEnvironment()
	if err != nil {
		return err
	}
	defer store.key.Close()
	value, kind, exists, err := store.Read("Path")
	if err != nil {
		return err
	}
	if !exists {
		kind = registry.EXPAND_SZ
	}
	if !remove && kind == registry.EXPAND_SZ && strings.Contains(dir, "%") {
		return fmt.Errorf("tool path containing %% cannot be stored in an expandable Windows PATH; choose a directory without %%")
	}
	if err := removeLegacyProfiles("JVM Tool PATH Configuration"); err != nil {
		return err
	}
	add, old := dir, ""
	if remove {
		add, old = "", dir
	}
	if err := store.Write("Path", UpdatePath(value, add, old, true), kind); err != nil {
		return err
	}
	return notifyEnvironment()
}

func clearWindowsJava(home string) error {
	store, err := openEnvironment()
	if err != nil {
		return err
	}
	defer store.key.Close()
	if err := clearJava(store, home); err != nil {
		return err
	}
	return notifyEnvironment()
}

func notifyEnvironment() error {
	text, err := syscall.UTF16PtrFromString("Environment")
	if err != nil {
		return err
	}
	proc := syscall.NewLazyDLL("user32.dll").NewProc("SendMessageTimeoutW")
	var result uintptr
	ok, _, callErr := proc.Call(0xffff, 0x001a, 0, uintptr(unsafe.Pointer(text)), 0x0002, 2000, uintptr(unsafe.Pointer(&result)))
	if ok == 0 {
		return fmt.Errorf("environment saved, but Windows notification failed; sign out and back in to refresh launching processes: %v", callErr)
	}
	return nil
}
