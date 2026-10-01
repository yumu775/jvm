package env

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// EnvironmentStore 允许用内存后端验证注册表事务，不接触用户环境。
type EnvironmentStore interface {
	Read(name string) (value string, kind uint32, exists bool, err error)
	Write(name, value string, kind uint32) error
	Delete(name string) error
}

func clearJava(store EnvironmentStore, home string) error {
	current, _, exists, err := store.Read("JAVA_HOME")
	if err != nil {
		return err
	}
	if !exists || !strings.EqualFold(filepath.Clean(current), filepath.Clean(home)) {
		return nil
	}
	path, pathKind, pathExists, err := store.Read("Path")
	if err != nil {
		return err
	}
	if pathExists {
		updated := UpdatePath(path, "", `%JAVA_HOME%\bin`, true)
		if err := store.Write("Path", UpdatePath(updated, "", filepath.Join(home, "bin"), true), pathKind); err != nil {
			return err
		}
	}
	if err := store.Delete("JAVA_HOME"); err != nil {
		if pathExists {
			return errors.Join(err, store.Write("Path", path, pathKind))
		}
		return err
	}
	return nil
}

type savedValue struct {
	value  string
	kind   uint32
	exists bool
}

func persistJava(store EnvironmentStore, home string) error {
	if err := validatePathEntry(home, true); err != nil {
		return err
	}
	names := []string{"JAVA_HOME", "Path"}
	saved := make([]savedValue, len(names))
	for i, name := range names {
		v, k, exists, err := store.Read(name)
		if err != nil {
			return err
		}
		saved[i] = savedValue{v, k, exists}
	}
	if (!saved[1].exists || saved[1].kind == 2) && strings.Contains(home, "%") {
		return fmt.Errorf("Java path containing %% cannot be stored in an expandable Windows PATH; choose a directory without %%")
	}
	oldBin := ""
	if saved[0].value != "" {
		oldBin = filepath.Join(saved[0].value, "bin")
	}
	javaPath := UpdatePath(saved[1].value, "", `%JAVA_HOME%\bin`, true)
	values := []string{home, UpdatePath(javaPath, filepath.Join(home, "bin"), oldBin, true)}
	for i, name := range names {
		kind := uint32(1)
		if i == 1 {
			kind = saved[i].kind
			if !saved[i].exists {
				kind = 2
			}
		}
		if err := store.Write(name, values[i], kind); err != nil {
			failures := []error{fmt.Errorf("write %s: %w", name, err)}
			for j := i - 1; j >= 0; j-- {
				var restore error
				if saved[j].exists {
					restore = store.Write(names[j], saved[j].value, saved[j].kind)
				} else {
					restore = store.Delete(names[j])
				}
				if restore != nil {
					failures = append(failures, fmt.Errorf("restore %s: %w", names[j], restore))
				}
			}
			return errors.Join(failures...)
		}
	}
	return nil
}
