//go:build windows

// 真机验显卡采集（性能计数器那条路）。默认跳过，手动跑时才真读。
// 跑法：$env:CATHERDER_REAL_GPU='1'; go test -count=1 -run TestGpuPercentReal -v ./internal/engine/
//
// TestGpuPercentReal —— 只读一次，看 known 是不是 true、数在不在 0~100
// TestGpuGateBlocksReal —— 自己起一段真 GPU 负载，验「连续两轮超阈值才判忙」真生效
// TestGpuGateFreesAfterReal —— 负载撤掉后闸门要能自己放开（别一直拦着）
package engine

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

// 真负载靠 python+torch（本机有）；没有就跳过，不算失败。
func realGpuLoad(t *testing.T) *exec.Cmd {
	t.Helper()
	py, err := exec.LookPath("python")
	if err != nil {
		t.Skip("本机没有 python，跑不了真负载")
	}
	script := os.Getenv("CATHERDER_GPU_LOAD_SCRIPT")
	if script == "" {
		t.Skip("设 CATHERDER_GPU_LOAD_SCRIPT=负载脚本路径 才跑真负载测试")
	}
	if _, err := os.Stat(script); err != nil {
		t.Skipf("负载脚本不在：%s", script)
	}
	c := exec.Command(py, script)
	hideWindow(c)
	if err := c.Start(); err != nil {
		t.Skipf("起负载失败: %v", err)
	}
	t.Cleanup(func() {
		if c.ProcessState == nil || !c.ProcessState.Exited() {
			_ = c.Process.Kill()
		}
	})
	// 等它把卡压起来（torch 导入 + CUDA 初始化就要十几秒），每 5 秒看一眼 nvidia-smi 到没到阈值。
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(5 * time.Second)
		if pct, ok := readNvidiaPercent(); ok && pct >= 60 {
			t.Logf("负载已起来（nvidia-smi 利用率 %d%%）", pct)
			return c
		}
	}
	_ = c.Process.Kill()
	t.Skipf("等了 90 秒显卡没压起来（python/torch 负载没跑成）")
	return c
}

func TestGpuPercentReal(t *testing.T) {
	if os.Getenv("CATHERDER_REAL_GPU") == "" {
		t.Skip("设 CATHERDER_REAL_GPU=1 才跑（要真显卡读数）")
	}
	p1, ok1 := readGpuPercent() // 第一次：建立基线
	time.Sleep(1200 * time.Millisecond)
	p2, ok2 := readGpuPercent() // 第二次：回上一段平均值
	t.Logf("第一次 pct=%d ok=%v；第二次 pct=%d ok=%v（ok 要为 true，第二次才是有效读数）", p1, ok1, p2, ok2)
	if !ok1 || !ok2 {
		t.Fatalf("性能计数器/nvidia-smi 都没读到数，known=false")
	}
	if p2 < 0 || p2 > 100 {
		t.Fatalf("利用率要在 0~100，拿到 %d", p2)
	}
}

// 真负载压着卡：闸门要在「连续两轮」之后判忙，并给出人话原因。
func TestGpuGateBlocksReal(t *testing.T) {
	if os.Getenv("CATHERDER_REAL_GPU") == "" {
		t.Skip("设 CATHERDER_REAL_GPU=1 才跑")
	}
	realGpuLoad(t)
	e := gpuTestEngine(t)
	if err := e.st.SetSetting("gpu_busy_percent", "30"); err != nil {
		t.Fatalf("设阈值: %v", err)
	}
	var lastPct int
	var lastBusy bool
	var lastWhy string
	for i := 0; i < 3; i++ {
		e.gpuAt = time.Now().Add(-time.Minute) // 每轮都重新读数（不复用上一轮的）
		busy, known, why := e.gpuBusy()
		lastBusy, lastWhy = busy, why
		lastPct = e.gpuPct
		t.Logf("第 %d 轮：利用率=%d%% 探得到=%v 判忙=%v 原因=[%s]", i+1, lastPct, known, busy, why)
		if !known {
			t.Fatalf("第 %d 轮读不到利用率（性能计数器与 nvidia-smi 都失败）", i+1)
		}
		if lastPct <= 30 {
			t.Fatalf("压卡中却只读到 %d%%（性能计数器没跟上真负载）", lastPct)
		}
		// 连轮规则：头一轮只计数不算忙，第二轮起才拦（单轮毛刺不算）。
		if i == 0 {
			if busy {
				t.Fatalf("头一轮就判忙（单轮毛刺不该算）")
			}
		} else if !busy {
			t.Fatalf("第 %d 轮还在超阈值却没判忙（连轮没累起来）", i+1)
		}
		time.Sleep(11 * time.Second)
	}
	if !lastBusy {
		t.Fatalf("连续三轮压卡却没判忙（利用率=%d%%）", lastPct)
	}
	if lastWhy == "" {
		t.Fatalf("判忙要写出原因")
	}
}

// 负载撤掉后：闸门要自己放开，不能一直拦着派发。
func TestGpuGateFreesAfterReal(t *testing.T) {
	if os.Getenv("CATHERDER_REAL_GPU") == "" {
		t.Skip("设 CATHERDER_REAL_GPU=1 才跑")
	}
	c := realGpuLoad(t)
	e := gpuTestEngine(t)
	if err := e.st.SetSetting("gpu_busy_percent", "30"); err != nil {
		t.Fatalf("设阈值: %v", err)
	}
	for i := 0; i < 2; i++ { // 先让它判成忙
		e.gpuAt = time.Now().Add(-time.Minute)
		e.gpuBusy()
		time.Sleep(11 * time.Second)
	}
	if busy, _, _ := e.gpuBusy(); !busy {
		t.Skip("压卡期间没判出忙，这条测不了「撤负载后放开」")
	}
	if c.ProcessState == nil || !c.ProcessState.Exited() {
		_ = c.Process.Kill()
	}
	time.Sleep(20 * time.Second) // 等利用率掉下来
	e.gpuAt = time.Now().Add(-time.Minute)
	busy, known, why := e.gpuBusy()
	e.gpuAt = time.Now().Add(-time.Minute)
	busy2, _, _ := e.gpuBusy()
	t.Logf("撤负载后：利用率=%d%% 探得到=%v 判忙=%v → 再一轮判忙=%v", e.gpuPct, known, busy, busy2)
	if busy || busy2 {
		t.Fatalf("负载已撤但闸门还拦着（利用率=%d%% 原因=[%s]）", e.gpuPct, why)
	}
}
