// 自动关机：纳入派工的窗口全空闲 + 队列空 + 服务空闲 + 复核通过
// → 先发通知 → 60 秒缓冲关机；期间有新活动自动撤销。
// 非 Windows 由入口程序决定不启用（autostart_other.go 同理，这里只写 Windows 分支）。
package engine

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"catherder/internal/openclaw"
)

const shutdownArmMs = 60 * 1000 // 60 秒缓冲，期间有新活动就撤销

// shutdownArmedAt 存「准备关机的时刻」，落盘：重启也不丢这个状态。
func (e *Engine) armedAt() int64 {
	v, _ := e.st.GetSetting("shutdown_armed_at")
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func (e *Engine) setArmed(at int64) { e.st.SetSetting("shutdown_armed_at", strconv.FormatInt(at, 10)) }

// ShutdownState 给界面显示：没启用 / 条件没满足 / 已准备关机（还有几秒）。
func (e *Engine) ShutdownState() string {
	if !e.flagOn("auto_shutdown", false) {
		return ""
	}
	at := e.armedAt()
	if at == 0 {
		return "等条件满足"
	}
	left := (at + shutdownArmMs - time.Now().UnixMilli()) / 1000
	if left < 0 {
		left = 0
	}
	return fmt.Sprintf("准备关机，还剩 %d 秒", left)
}

// tickShutdown 每轮检查一次关机条件。dryRun 不动任何东西。
func (e *Engine) tickShutdown(included map[string]bool, wins map[string]openclaw.Window, dryRun bool) {
	if !e.flagOn("auto_shutdown", false) {
		if e.armedAt() != 0 {
			e.setArmed(0)
		}
		return
	}
	if !e.inShutdownWindow() {
		if e.armedAt() != 0 {
			e.disarm("不在设定的关机时段")
		}
		return
	}
	ok, why := e.shutdownReady(included, wins)
	if !ok {
		if e.armedAt() != 0 {
			e.disarm(why)
		}
		return
	}
	now := time.Now().UnixMilli()
	at := e.armedAt()
	if at == 0 {
		if dryRun {
			e.out("[dry] 条件满足，会准备关机（60 秒缓冲）")
			return
		}
		e.setArmed(now)
		e.out("队列清空且全空闲，60 秒后自动关机（期间有新活动自动撤销）")
		e.notify("🌙 工头派工台：全部任务已完成且所有窗口空闲，60 秒后自动关机。如需取消请在界面取消，或产生新的窗口活动。")
		return
	}
	if now < at+shutdownArmMs {
		return // 还在缓冲里
	}
	if dryRun {
		e.out("[dry] 缓冲结束，会执行关机")
		return
	}
	e.out("执行关机（shutdown /s /t 0 /f）")
	e.st.Log("shutdown", "队列清空且全空闲，执行关机")
	// /f = 强制：直接关掉挡住关机的程序。不加它，Windows 碰到「有程序阻止关机」会弹框等人点
	// 命令其实已经下达、机器却一直没关。
	cmd := exec.Command("shutdown", "/s", "/t", "0", "/f")
	if err := cmd.Run(); err != nil {
		e.disarm("关机命令失败: " + err.Error())
		e.notify("⚠️ 工头派工台：自动关机命令失败了（" + err.Error() + "），已撤销，机器还开着。")
	}
}

// CancelShutdown 给界面「取消关机」按钮用：撤掉这次待关机，同时把「自动关机」的勾选取消并保存。
// 勾选一关，ShutdownState 就返回空 → 横幅立刻消失，设置页的勾选也同步变掉。
func (e *Engine) CancelShutdown() {
	if e.armedAt() != 0 {
		e.disarm("用户在界面点击取消")
	}
	e.st.SetSetting("auto_shutdown", "0")
}

// ResetArmAfterBoot 程序启动时调一次：开机/重启软件后，自动关机整套先取消。
// ① 落盘的「准备关机」标记随开机已失效，留着会让界面一开机就挂倒计时。
// ② 开关本身也取消：刚开机时队列必然是空的、窗口也必然是空闲的，条件秒满足，
//
//	不取消就会在开机两三分钟后再下一次关机。要用的话在设置里重新勾选。
func (e *Engine) ResetArmAfterBoot() {
	if e.armedAt() != 0 {
		e.disarm("程序重启，上次挂着的关机准备作废")
	}
	if e.flagOn("auto_shutdown", false) {
		e.st.SetSetting("auto_shutdown", "0")
		e.out("启动时自动取消自动关机（需要请在设置里重新勾选）")
		e.st.Log("shutdown", "启动时自动取消自动关机")
	}
}

// disarm 撤销关机：已发过 shutdown 就补一条 /a 撤销。
func (e *Engine) disarm(reason string) {
	if e.armedAt() == 0 {
		return
	}
	e.setArmed(0)
	e.out("自动关机已撤销：" + reason)
	e.st.Log("shutdown", "撤销：%s", reason)
	exec.Command("shutdown", "/a").Run()
}

// shutdownReady 判关机条件。返回不满足的原因给日志看。
func (e *Engine) shutdownReady(included map[string]bool, wins map[string]openclaw.Window) (bool, string) {
	c, err := e.st.Counts()
	if err != nil {
		return false, "读任务数失败"
	}
	// 「正在改」的那条也算未完成：用户编辑期间不判队列空去关机。
	if c["queued"]+c["dispatched"]+c["paused"]+c["editing"] > 0 {
		return false, "仍有任务未完成"
	}
	if c["error"] > 0 {
		return false, "还有任务标着异常，没处理"
	}
	any := false
	for key, w := range wins {
		if !included[key] {
			continue
		}
		any = true
		if !w.Idle() || w.Pending > 0 {
			return false, "窗口 " + displayName(w) + " 仍在执行任务"
		}
		if time.Now().UnixMilli()-w.UpdatedAt < freshInputMs {
			return false, "窗口 " + displayName(w) + " 刚有新输入"
		}
	}
	if !any {
		return false, "未勾选任何可派工窗口"
	}
	if e.flagOn("gate_service", true) {
		if a := e.serviceBusy(); a.Known && a.Busy {
			return false, "模型服务还在忙"
		}
	}
	// 显卡还在动也算没空闲。
	if e.flagOn("gate_gpu", true) {
		if busy, known, why := e.gpuBusy(); known && busy {
			return false, why
		}
	}
	return true, ""
}

// inShutdownWindow 可选高级项：只在 HH:MM–HH:MM 之间关（默认不限时段）。
func (e *Engine) inShutdownWindow() bool {
	v, _ := e.st.GetSetting("shutdown_hours")
	v = strings.TrimSpace(v)
	if v == "" {
		return true
	}
	parts := strings.SplitN(v, "-", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return true // 写歪了就当不限，别因此误关机
	}
	lo, hi := parts[0], parts[1]
	now := time.Now().Format("15:04")
	norm := func(s string) string {
		var h, m int
		if n, _ := fmt.Sscanf(strings.TrimSpace(s), "%d:%d", &h, &m); n != 2 {
			return ""
		}
		return fmt.Sprintf("%02d:%02d", h, m)
	}
	a, b := norm(lo), norm(hi)
	if a == "" || b == "" {
		return true
	}
	if a <= b {
		return now >= a && now <= b
	}
	return now >= a || now <= b // 跨零点，如 22:00-06:00
}
