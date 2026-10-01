//go:build !windows

package env

import "fmt"

func ReadPersistentEnvironment() (map[string]string, error) { return map[string]string{}, nil }

func persistWindowsJava(string) error           { return fmt.Errorf("Windows registry is unavailable") }
func persistWindowsToolPath(string, bool) error { return fmt.Errorf("Windows registry is unavailable") }
func clearWindowsJava(string) error             { return fmt.Errorf("Windows registry is unavailable") }
