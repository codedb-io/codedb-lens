//go:build !windows

package main

// Linux/macOS builds are CLI only (servers and Docker). No args prints usage.
func runGUI() bool { return false }

func attachConsole() {}
