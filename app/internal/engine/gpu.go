// 显卡忙闲探测。
// 背景：派工台原来只看「目标窗口那一回合结束没结束」+「那个窗口用的模型服务报不报忙」
// 看不见外部进程或别的活占着显卡 —— 「绘图员h 被误判闲置」就是这么来的。
package engine

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// gpuHotRounds 连续几轮超过阈值才算忙。
const gpuHotRounds = 2

// gpuProbe 是「读一次显卡利用率」这件事本身；测试里换成假读数，不去动真显卡。
var gpuProbe = readGpuPercent

// gpuBusy 看显卡忙不忙，回（忙？、探得到？、拦下的原因）。
// 探不到（没装 nvidia-smi、不是 NVIDIA 卡）→ known=false，调用方不拦，跟原来一样只看窗口与服务。
func (e *Engine) gpuBusy() (bool, bool, string) {
	limit := e.gpuLimit()
	// 同一轮里派发和自动关机都要判：10 秒内复用刚才那次读数，别把一轮数成「连续两轮」。
	if time.Since(e.gpuAt) >= 10*time.Second {
		pct, ok := gpuProbe()
		e.gpuAt, e.gpuKnown, e.gpuPct = time.Now(), ok, pct
		switch {
		case !ok, pct <= limit:
			e.gpuHot = 0
		default:
			e.gpuHot++
		}
	}
	if !e.gpuKnown {
		return false, false, ""
	}
	if e.gpuHot < gpuHotRounds {
		return false, true, ""
	}
	return true, true, fmt.Sprintf("显卡正忙（利用率 %d%%，连续 %d 轮超过 %d%%）", e.gpuPct, gpuHotRounds, limit)
}

// gpuLimit 读阈值（设置里的「显卡利用率阈值」，默认 30）。写歪了也当 30，别因此不拦或乱拦。
func (e *Engine) gpuLimit() int {
	v := e.settingInt("gpu_busy_percent", 30)
	if v < 5 || v > 95 {
		return 30
	}
	return v
}

// readGpuPercent 平台相关（gpu_windows.go / gpu_other.go）
// Windows 走性能计数器（厂商中立，NVIDIA/AMD/Intel 都有），读不到再退回 nvidia-smi；
// 其它系统走 nvidia-smi。

// readNvidiaPercent 跑一次 nvidia-smi，回利用率最高那张卡的百分比（几张卡取最高的，整机资源看最紧的那张）。
func readNvidiaPercent() (int, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=utilization.gpu", "--format=csv,noheader,nounits")
	hideWindow(c) // 界面版没有控制台，不藏会每轮闪一个黑窗
	out, err := c.Output()
	if err != nil {
		return 0, false
	}
	top, found := 0, false
	for _, line := range strings.Split(string(out), "\n") {
		v, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			continue
		}
		if !found || v > top {
			top, found = v, true
		}
	}
	return top, found
}
