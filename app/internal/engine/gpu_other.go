//go:build !windows

package engine

import "os/exec"

// hideWindow 非 Windows 没有控制台窗这个问题，空着。
func hideWindow(c *exec.Cmd) {}

// readGpuPercent 非 Windows：走 nvidia-smi（Linux 还有 /sys、rocm-smi 等路，以后要支持再加）。
func readGpuPercent() (int, bool) { return readNvidiaPercent() }
