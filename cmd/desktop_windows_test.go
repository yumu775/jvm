//go:build windows

package cmd

import "testing"

func TestDesktopSetupRequiresConfirmation(t *testing.T) {
	for _, input := range []string{"", "\r\n", "no", "anything"} {
		if desktopSetupConfirmed(input) {
			t.Fatalf("unexpected confirmation: %q", input)
		}
	}
	for _, input := range []string{"Y\r\n", "yes", "是", "确认"} {
		if !desktopSetupConfirmed(input) {
			t.Fatalf("confirmation rejected: %q", input)
		}
	}
}
