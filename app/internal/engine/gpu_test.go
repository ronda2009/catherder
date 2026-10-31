// 显卡闸的测试（老板 2026-10-27 批「要加」）：
// 1) 连续两轮超阈值才算忙（单轮尖峰不算）；2) 低于阈值立刻清零；3) 读不到就不拦。
// gpuProbe 是假读数，测试不碰真显卡。
package engine

import (
	"strings"
	"testing"
	"time"

	"catherder/internal/openclaw"
	"catherder/internal/store"
)

func gpuTestEngine(t *testing.T) *Engine {
	t.Helper()
	db := t.TempDir() + "/g.sqlite"
	st, err := store.Open(db)
	if err != nil {
		t.Fatalf("开库: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	e := New(nil, st, db)
	e.SetOut(func(string) {})
	return e
}

// 把读数换成固定值，并把上次读数时刻拨到一分钟前（绕过「10 秒内复用刚才那次」）。
func fakeGpu(t *testing.T, e *Engine, pct int, ok bool) {
	t.Helper()
	old := gpuProbe
	gpuProbe = func() (int, bool) { return pct, ok }
	e.gpuAt = time.Now().Add(-time.Minute)
	t.Cleanup(func() { gpuProbe = old })
}

// 本轮重新读数（不复用上一轮）。
func regpu(e *Engine) { e.gpuAt = time.Now().Add(-time.Minute) }

// fakeWS 塞一个空网关连接：dryRun 派发只看连接在不在、不真发消息，
// 不塞就会去连真网关（测试机没配网关地址就整轮不派，测不到闸门）。
func fakeWS(e *Engine) { e.ws = &openclaw.Client{} }

func TestGpuBusyNeedsTwoRounds(t *testing.T) {
	e := gpuTestEngine(t)
	fakeGpu(t, e, 80, true) // 80% > 默认阈值 30%

	if busy, known, _ := e.gpuBusy(); busy || !known {
		t.Fatalf("第一轮就该不算忙（单轮尖峰不拦），得 busy=%v known=%v", busy, known)
	}
	regpu(e) // 下一轮（隔得够久，重新读数）
	busy, _, why := e.gpuBusy()
	if !busy {
		t.Fatal("第二轮还在 80%，该拦下")
	}
	if why == "" {
		t.Fatal("拦下来要给原因写进日志")
	}
}

func TestGpuBusyClearsWhenCool(t *testing.T) {
	e := gpuTestEngine(t)
	fakeGpu(t, e, 80, true)
	e.gpuBusy()
	regpu(e)
	if busy, _, _ := e.gpuBusy(); !busy {
		t.Fatal("连续两轮 80% 该判忙")
	}
	gpuProbe = func() (int, bool) { return 5, true }
	regpu(e)
	if busy, _, _ := e.gpuBusy(); busy {
		t.Fatal("降到 5% 第一轮就该清零、不拦")
	}
	regpu(e)
	if busy, _, _ := e.gpuBusy(); busy {
		t.Fatal("降到 5% 第二轮也不该拦")
	}
}

func TestGpuBusyUnknownDoesNotBlock(t *testing.T) {
	e := gpuTestEngine(t)
	fakeGpu(t, e, 0, false) // 读不到（没装 nvidia-smi）
	for i := 0; i < 3; i++ {
		if busy, known, _ := e.gpuBusy(); busy || known {
			t.Fatalf("读不到就不该拦派发（第 %d 次）busy=%v known=%v", i+1, busy, known)
		}
	}
}

// 闸门真接上往派发：显卡连轮超阈值 → 那条不派，日志里要写出原因。
func TestDispatchHeldWhenGpuBusy(t *testing.T) {
	e := newTestEngine(t) // 助手默认关了服务/显卡闸门
	e.st.SetSetting("gate_gpu", "1")
	fakeWS(e)
	fakeGpu(t, e, 84, true)
	if _, err := e.st.AddTask("显卡闸测试正文", "w-a", "A"); err != nil {
		t.Fatalf("造任务: %v", err)
	}
	inc := map[string]bool{"w-a": true}
	idle := map[string]openclaw.Window{"w-a": win("w-a", "A", "strata", true)}

	var lines []string
	e.SetOut(func(s string) { lines = append(lines, s) })
	if err := e.dispatch(idle, inc, true); err != nil { // 第一轮：单轮尖峰，该照派
		t.Fatalf("第一轮派发: %v", err)
	}
	if !sent(lines, "T-001") {
		t.Fatalf("单轮尖峰不该拦，得派出去，日志=%v", lines)
	}

	lines = nil
	regpu(e) // 第二轮仍在 84%
	if err := e.dispatch(idle, inc, true); err != nil {
		t.Fatalf("第二轮派发: %v", err)
	}
	if sent(lines, "T-001") {
		t.Fatalf("连轮 84%% 该拦住，日志=%v", lines)
	}
	if !hasGpuReason(lines) {
		t.Fatalf("拦下来日志要写明是显卡忙，日志=%v", lines)
	}
}

// 闸门关掉（或读不到显卡）→ 不拦，跟原来一样。
func TestDispatchProceedsWithoutGpuGate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		gate  string
		probe func() (int, bool)
	}{
		{"闸门关了", "0", func() (int, bool) { return 99, true }},
		{"读不到显卡", "1", func() (int, bool) { return 0, false }},
	} {
		e := newTestEngine(t)
		e.st.SetSetting("gate_gpu", tc.gate)
		fakeWS(e)
		old := gpuProbe
		gpuProbe = tc.probe
		e.gpuAt = time.Now().Add(-time.Minute)
		if _, err := e.st.AddTask("显卡闸测试正文", "w-a", "A"); err != nil {
			t.Fatalf("%s 造任务: %v", tc.name, err)
		}
		var lines []string
		e.SetOut(func(s string) { lines = append(lines, s) })
		for i := 0; i < 2; i++ {
			e.gpuAt = time.Now().Add(-time.Minute)
			if err := e.dispatch(map[string]openclaw.Window{"w-a": win("w-a", "A", "strata", true)}, map[string]bool{"w-a": true}, true); err != nil {
				t.Fatalf("%s 第 %d 轮: %v", tc.name, i+1, err)
			}
		}
		if !sent(lines, "T-001") {
			t.Errorf("%s 不该拦派发，日志=%v", tc.name, lines)
		}
		gpuProbe = old
	}
}

func sent(lines []string, id string) bool {
	for _, l := range lines {
		if strings.Contains(l, "将把 "+id) {
			return true
		}
	}
	return false
}

func hasGpuReason(lines []string) bool {
	for _, l := range lines {
		if strings.Contains(l, "显卡正忙") {
			return true
		}
	}
	return false
}

func TestGpuLimitReadsSetting(t *testing.T) {
	e := gpuTestEngine(t)
	if got := e.gpuLimit(); got != 30 {
		t.Fatalf("没设置时默认该是 30，得 %d", got)
	}
	e.st.SetSetting("gpu_busy_percent", "70")
	if got := e.gpuLimit(); got != 70 {
		t.Fatalf("设置里写 70 就该用 70，得 %d", got)
	}
	e.st.SetSetting("gpu_busy_percent", "写歪了")
	if got := e.gpuLimit(); got != 30 {
		t.Fatalf("写歪了该退回 30，得 %d", got)
	}
}
