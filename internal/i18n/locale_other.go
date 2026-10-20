//go:build !windows

package i18n

func osLocale() string { return "" } // LANG/LC_ALL already checked by Detect
