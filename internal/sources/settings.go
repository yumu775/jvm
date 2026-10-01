package sources

import (
	"encoding/json"
	"fmt"
	"jvm/internal/config"
	"os"
	"path/filepath"
	"time"
)

type sourceSettings struct {
	Enabled    map[string]bool `json:"enabled"`
	Priorities map[string]int  `json:"priorities"`
	Default    string          `json:"default_source"`
}

func sourceConfigPath() (string, error) {
	root, err := config.GetJVMDir()
	return filepath.Join(root, "sources.json"), err
}
func readSourceSettings() (sourceSettings, error) {
	s := sourceSettings{Enabled: map[string]bool{}, Priorities: map[string]int{}, Default: "adoptium"}
	path, err := sourceConfigPath()
	if err != nil {
		return s, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	var raw map[string]json.RawMessage
	if err = json.Unmarshal(data, &raw); err != nil {
		return s, fmt.Errorf("invalid sources configuration: %w", err)
	}
	if _, modern := raw["enabled"]; modern {
		err = json.Unmarshal(data, &s)
	} else {
		err = json.Unmarshal(data, &s.Enabled)
	}
	if err != nil {
		return s, fmt.Errorf("invalid sources configuration: %w", err)
	}
	if s.Enabled == nil {
		s.Enabled = map[string]bool{}
	}
	if s.Priorities == nil {
		s.Priorities = map[string]int{}
	}
	if s.Default == "" {
		s.Default = "adoptium"
	}
	return s, nil
}
func IsSupportedSource(s JavaSource) bool {
	switch s.APIType {
	case "adoptium", "zulu", "corretto", "graalvm":
		return true
	}
	return false
}
func (sm *SourceManager) LoadSources() ([]JavaSource, error) {
	settings, err := readSourceSettings()
	if err != nil {
		return nil, err
	}
	all := sm.GetDefaultSources()
	for i := range all {
		if v, ok := settings.Enabled[all[i].Name]; ok {
			all[i].Enabled = v
		}
		if v, ok := settings.Priorities[all[i].Name]; ok {
			all[i].Priority = v
		}
	}
	return all, nil
}
func updateSourceSettings(change func(*sourceSettings) error) error {
	path, err := sourceConfigPath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	lockPath := path + ".lock"
	for {
		f, e := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e == nil {
			f.Close()
			break
		}
		if !os.IsExist(e) {
			return e
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("sources configuration locked: %s", lockPath)
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer os.Remove(lockPath)
	settings, err := readSourceSettings()
	if err != nil {
		return err
	}
	if err = change(&settings); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".sources-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func (sm *SourceManager) sourceByName(name string) (JavaSource, error) {
	for _, s := range sm.GetDefaultSources() {
		if s.Name == name {
			return s, nil
		}
	}
	return JavaSource{}, fmt.Errorf("unknown source: %s", name)
}
func (sm *SourceManager) SetEnabled(name string, enabled bool) error {
	s, err := sm.sourceByName(name)
	if err != nil {
		return err
	}
	if enabled && !IsSupportedSource(s) {
		return fmt.Errorf("source %s is not supported for automatic downloads; import manually", name)
	}
	return updateSourceSettings(func(c *sourceSettings) error { c.Enabled[name] = enabled; return nil })
}
func (sm *SourceManager) DefaultSource() (string, error) {
	s, err := readSourceSettings()
	return s.Default, err
}
func (sm *SourceManager) SetDefault(name string) error {
	s, err := sm.sourceByName(name)
	if err != nil {
		return err
	}
	if !IsSupportedSource(s) {
		return fmt.Errorf("source %s is not supported for automatic downloads; import manually", name)
	}
	return updateSourceSettings(func(c *sourceSettings) error {
		enabled := s.Enabled
		if v, ok := c.Enabled[name]; ok {
			enabled = v
		}
		if !enabled {
			return fmt.Errorf("source %s is disabled", name)
		}
		c.Default = name
		return nil
	})
}
func (sm *SourceManager) SetPriority(name string, n int) error {
	if _, err := sm.sourceByName(name); err != nil {
		return err
	}
	if n < 0 {
		return fmt.Errorf("priority must be nonnegative")
	}
	return updateSourceSettings(func(c *sourceSettings) error { c.Priorities[name] = n; return nil })
}
