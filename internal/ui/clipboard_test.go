package ui

import (
	"errors"
	"testing"
)

func TestWriteClipboardForOSUsesPBcopyOnMacOS(t *testing.T) {
	var nativeText string
	terminalCalled := false

	err := writeClipboardForOS("note body", "darwin", func(text string) error {
		nativeText = text
		return nil
	}, func(string) error {
		terminalCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("write clipboard: %v", err)
	}
	if nativeText != "note body" {
		t.Fatalf("native clipboard text = %q, want note body", nativeText)
	}
	if terminalCalled {
		t.Fatal("terminal clipboard called after successful macOS copy")
	}
}

func TestWriteClipboardForOSFallsBackToTerminalOnMacOS(t *testing.T) {
	terminalText := ""

	err := writeClipboardForOS("note body", "darwin", func(string) error {
		return errors.New("pbcopy unavailable")
	}, func(text string) error {
		terminalText = text
		return nil
	})
	if err != nil {
		t.Fatalf("write clipboard: %v", err)
	}
	if terminalText != "note body" {
		t.Fatalf("terminal clipboard text = %q, want note body", terminalText)
	}
}

func TestWriteClipboardForOSUsesTerminalOnLinux(t *testing.T) {
	nativeCalled := false
	terminalText := ""

	err := writeClipboardForOS("note body", "linux", func(string) error {
		nativeCalled = true
		return nil
	}, func(text string) error {
		terminalText = text
		return nil
	})
	if err != nil {
		t.Fatalf("write clipboard: %v", err)
	}
	if nativeCalled {
		t.Fatal("macOS clipboard called on Linux")
	}
	if terminalText != "note body" {
		t.Fatalf("terminal clipboard text = %q, want note body", terminalText)
	}
}
