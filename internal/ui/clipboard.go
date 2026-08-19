package ui

import (
	"encoding/base64"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type clipboardWriter func(string) error

func writePlatformClipboard(text string) error {
	return writeClipboardForOS(text, runtime.GOOS, writeMacOSClipboard, writeOSC52Clipboard)
}

func writeClipboardForOS(text, goos string, macOSCopy, terminalCopy clipboardWriter) error {
	if goos == "darwin" {
		if err := macOSCopy(text); err == nil {
			return nil
		}
	}
	return terminalCopy(text)
}

func writeMacOSClipboard(text string) error {
	cmd := exec.Command("/usr/bin/pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func writeOSC52Clipboard(text string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	sequence := "\x1b]52;c;" + encoded + "\a"

	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err == nil {
		defer func() { _ = tty.Close() }()
		_, err = tty.WriteString(sequence)
		return err
	}

	_, err = os.Stdout.WriteString(sequence)
	return err
}
