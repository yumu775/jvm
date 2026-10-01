//go:build !windows

package cmd

func executeDesktopLaunch() (bool, error) { return false, nil }
