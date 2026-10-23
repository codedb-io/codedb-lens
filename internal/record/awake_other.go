//go:build !windows

package record

// stayAwake: servers and containers don't sleep; on a Linux desktop use systemd-inhibit.
func stayAwake() {}
