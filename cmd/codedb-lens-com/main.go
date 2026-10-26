//go:build windows

// codedb-lens.com is the console twin of codedb-lens.exe, which is a GUI-subsystem binary: terminals do not wait for
// GUI programs, so the shell and the Lens would read the keyboard at the same time (mcp allow-write, key register).
// Typed as "codedb-lens", Windows picks .com before .exe (PATHEXT); this waits for the .exe next to it, on the same
// console, and returns its exit code.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
)

func main() {
	self, err := os.Executable()
	if err != nil {
		fail(err)
	}
	cmd := exec.Command(strings.TrimSuffix(self, filepath.Ext(self))+".exe", os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if len(os.Args) == 1 { // no arguments opens the app: give the terminal back
		if err := cmd.Start(); err != nil {
			fail(err)
		}
		return
	}
	signal.Ignore(os.Interrupt) // Ctrl+C reaches the .exe too (same console); exit when it does
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.ExitCode())
		}
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
