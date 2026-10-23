package record

import "golang.org/x/sys/windows"

const esSystemRequired = 0x00000001

var setThreadExecutionState = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetThreadExecutionState")

// stayAwake resets the Windows sleep timer; called every cycle while recording (the display may still turn off).
// ponytail: ES_SYSTEM_REQUIRED alone, once per cycle, instead of ES_CONTINUOUS: that one belongs to the calling OS
// thread and needs a locked goroutine kept alive. The shortest sleep timeout Windows offers (1 min) is 4 cycles.
func stayAwake() {
	if setThreadExecutionState.Find() == nil {
		setThreadExecutionState.Call(esSystemRequired)
	}
}
