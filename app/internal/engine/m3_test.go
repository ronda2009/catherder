// M3 三块逻辑的单元测试：同模型并发确认、全异常停机、自动关机条件。
// 不连网关、不连模型服务，只用临时库 + 假窗口，跑起来不占模型。
package engine

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"catherder/internal/openclaw"
	"catherder/internal/store"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("开测试库失败: %v", err)
	}
	t.Cleanup(st.Close)
	cfg := &openclaw.Config{}
	cfg.Models.Providers = map[string]openclaw.ProviderEntry{
		"strata": {BaseURL: "http://127.0.0.1:8095/v1"},
		"cloud":  {BaseURL: "https://api.example.com/v1"},
	}
	e := New(cfg, st, "")
	e.SetOut(func(string) {}) // 测试里不刷屏
	// 关掉服务闸门：不然测试结果会跟着本机模型服务忙闲变（应该只验逻辑）。
	st.SetSetting("gate_service", "0")
	// 显卡闸门同理（不关会去跑真 nvidia-smi，显卡一忙测试就跟着抖）。
	st.SetSetting("gate_gpu", "0")
	return e
}

func win(key, label, provider string, idle bool) openclaw.Window {
	st := "running"
	if idle {
		st = "done"
	}
	return openclaw.Window{Key: key, ID: key, Label: label, Status: st, Provider: provider, UpdatedAt: 0}
}

func TestSameModelConflict(t *testing.T) {
	e := newTestEngine(t)
	running := win("w-run", "执行中的窗口", "strata", false)
	same := win("w-same", "同模型窗口", "strata", true)
	cloud := win("w-cloud", "云端窗口", "cloud", true)
	ds := []store.Task{{ID: 1, TargetKey: "w-run"}}
	wins := map[string]openclaw.Window{"w-run": running, "w-same": same, "w-cloud": cloud}

	if r := e.sameModelConflict(store.Task{ID: 2, TargetKey: "w-same"}, same, ds, wins); r == nil {
		t.Error("同模型并发应该要问，结果没问")
	} else if r.Endpoint != "127.0.0.1:8095" {
		t.Errorf("端口应该报 127.0.0.1:8095，实际 %s", r.Endpoint)
	}
	if r := e.sameModelConflict(store.Task{ID: 3, TargetKey: "w-cloud"}, cloud, ds, wins); r != nil {
		t.Error("云端模型探不到并发能力，不该弹，结果弹了")
	}
	if r := e.sameModelConflict(store.Task{ID: 4, TargetKey: "w-same"}, same, nil, wins); r != nil {
		t.Error("无执行中任务时不应弹出确认，实际弹出了")
	}
}

func TestConfirmMuteAndDecision(t *testing.T) {
	e := newTestEngine(t)
	e.setConfirmPending(ConfirmRequest{TaskID: 7, Question: "q"})
	r := e.PendingConfirm()
	if r == nil || r.TaskID != 7 {
		t.Fatalf("挂起的问题读不出来: %+v", r)
	}
	// 暂不派发 + 今天不再提醒
	if err := e.AnswerConfirm(false, "today"); err != nil {
		t.Fatalf("答复失败: %v", err)
	}
	if e.PendingConfirm() != nil {
		t.Error("答复后应清除挂起的确认项")
	}
	if !e.confirmMuted() {
		t.Error("勾了今天不再提醒，这一轮就该静音")
	}
	if _, skip := e.confirmDecision(7); !skip {
		t.Error("选了先不发，这条任务本轮就该跳过")
	}
	if _, skip := e.confirmDecision(8); skip {
		t.Error("用户的表态只针对那一条，别误伤其它任务")
	}
	// 同意并发：一次性凭证，用完即清
	e.st.SetSetting("mute_mode", "")
	e.setConfirmPending(ConfirmRequest{TaskID: 9})
	if err := e.AnswerConfirm(true, ""); err != nil {
		t.Fatalf("答复失败: %v", err)
	}
	if g, _ := e.confirmDecision(9); !g {
		t.Error("用户同意了，这条就该放行")
	}
	if g, _ := e.confirmDecision(9); g {
		t.Error("同意是一次性的，第二条任务要重新问")
	}
	// 过期作废
	e.st.SetSetting("confirm_skip", "11|1")
	if _, skip := e.confirmDecision(11); skip {
		t.Error("跳过凭证过期了就该作废")
	}
}

func TestCheckHalt(t *testing.T) {
	e := newTestEngine(t)
	add := func(state string) int {
		id, err := e.st.AddTask("正文", "w-same", "同模型窗口")
		if err != nil {
			t.Fatalf("加任务失败: %v", err)
		}
		if state == "error" {
			if err := e.st.MarkError(id, "模型服务不通"); err != nil {
				t.Fatalf("标异常失败: %v", err)
			}
		}
		return id
	}
	e.checkHalt()
	if h, _ := e.st.GetSetting("halt"); h != "" {
		t.Error("队列空时不该停机")
	}
	add("error")
	e.checkHalt()
	if h, _ := e.st.GetSetting("halt"); h == "" {
		t.Error("队列全部异常时应停止派发等待处理")
	}
	e.checkHalt() // 重复跑不该叠加
	if h, _ := e.st.GetSetting("halt"); h == "" {
		t.Error("停机标记不该自己消失")
	}
	add("queued") // 又排了一条能派的
	e.checkHalt()
	if h, _ := e.st.GetSetting("halt"); h != "" {
		t.Errorf("还有能派的任务就该自动恢复，实际还停着: %s", h)
	}
	if err := e.st.RetryTask(1); err != nil {
		t.Fatalf("重试失败: %v", err)
	}
	if q, _ := e.st.Queue(); len(q) != 2 {
		t.Errorf("重试后该回到队列，实际 %d 条", len(q))
	}
}

func TestShutdownReady(t *testing.T) {
	e := newTestEngine(t)
	inc := map[string]bool{"w-a": true, "w-b": true}
	idle := map[string]openclaw.Window{"w-a": win("w-a", "A", "strata", true), "w-b": win("w-b", "B", "strata", true)}

	if ok, _ := e.shutdownReady(inc, idle); !ok {
		t.Error("全空闲且队列空，该满足关机条件")
	}
	busy := map[string]openclaw.Window{"w-a": win("w-a", "A", "strata", true), "w-b": win("w-b", "B", "strata", false)}
	if ok, why := e.shutdownReady(inc, busy); ok {
		t.Error("仍有窗口在执行任务，不应满足关机条件")
	} else if why == "" {
		t.Error("该给出不满足的原因")
	}
	if ok, _ := e.shutdownReady(map[string]bool{}, idle); ok {
		t.Error("一个窗口都没勾，不该关机（免得啥都没干就关）")
	}
	// 仍有任务未完成 / 异常未处理，均不应关机
	id, _ := e.st.AddTask("正文", "w-a", "A")
	if ok, _ := e.shutdownReady(inc, idle); ok {
		_ = id
		t.Error("还有排队任务，不该满足关机条件")
	}
	if err := e.st.MarkDispatched(id); err != nil {
		t.Fatalf("标记派发失败: %v", err)
	}
	if ok, _ := e.shutdownReady(inc, idle); ok {
		t.Error("还有在跑的任务，不该满足关机条件")
	}
	if err := e.st.MarkError(id, "炸了"); err != nil {
		t.Fatalf("标异常失败: %v", err)
	}
	if ok, _ := e.shutdownReady(inc, idle); ok {
		t.Error("有异常没处理，不该关机")
	}
	if err := e.st.Cancel(id); err != nil {
		t.Fatalf("撤销失败：%v", err)
	}
	if ok, _ := e.shutdownReady(inc, idle); !ok {
		t.Error("撤销异常任务后应重新满足关机条件")
	}
}

func TestShutdownHours(t *testing.T) {
	e := newTestEngine(t)
	if !e.inShutdownWindow() {
		t.Error("没设时段就该不限")
	}
	e.st.SetSetting("shutdown_hours", "22:00-06:00")
	now := time.Now().Format("15:04")
	want := now >= "22:00" || now <= "06:00"
	if got := e.inShutdownWindow(); got != want {
		t.Errorf("跨零点时段判断错了：现在 %s，应该 %v，实际 %v", now, want, got)
	}
	e.st.SetSetting("shutdown_hours", "09:30-17:00")
	now = time.Now().Format("15:04")
	want2 := now >= "09:30" && now <= "17:00"
	if got := e.inShutdownWindow(); got != want2 {
		t.Errorf("白天时段判断错了：现在 %s，应该 %v，实际 %v", now, want2, got)
	}
	e.st.SetSetting("shutdown_hours", "瞎写的")
	if !e.inShutdownWindow() {
		t.Error("写歪了该当成不限，不能因此误关机")
	}
}

func TestTickShutdownArmsAndDisarms(t *testing.T) {
	e := newTestEngine(t)
	e.st.SetSetting("notify_channel", "") // 测试里不发真通知
	inc := map[string]bool{"w-a": true}
	idle := map[string]openclaw.Window{"w-a": win("w-a", "A", "strata", true)}

	// 没勾选启用：不该武装
	e.tickShutdown(inc, idle, false)
	if e.armedAt() != 0 {
		t.Error("没开自动关机不该武装")
	}
	e.st.SetSetting("auto_shutdown", "1")
	e.tickShutdown(inc, idle, false)
	if e.armedAt() == 0 {
		t.Error("全空闲且队列空，该武装准备关机")
	}
	if s := e.ShutdownState(); s == "" {
		t.Error("界面该看得到「准备关机」")
	}
	// 窗口又忙了 → 自动撤销
	busy := map[string]openclaw.Window{"w-a": win("w-a", "A", "strata", false)}
	e.tickShutdown(inc, busy, false)
	if e.armedAt() != 0 {
		t.Error("窗口又忙了该自动撤销关机")
	}
	// 时段不匹配 → 不武装也不留旧标记
	e.st.SetSetting("shutdown_hours", "03:00-03:01")
	e.tickShutdown(inc, idle, false)
	if e.armedAt() != 0 {
		t.Error("不在设定时段不该武装")
	}
	// dryRun 不该改任何状态
	e.st.SetSetting("shutdown_hours", "")
	e.tickShutdown(inc, idle, true)
	if e.armedAt() != 0 {
		t.Error("dryRun 不该武装")
	}
}

// 重启后不该继续挂着上次落盘的关机准备。
func TestResetArmAfterBoot(t *testing.T) {
	e := newTestEngine(t)
	e.st.SetSetting("notify_channel", "")
	e.st.SetSetting("auto_shutdown", "1")
	if e.armedAt() != 0 {
		t.Error("新库不该有旧的关机准备")
	}
	e.setArmed(time.Now().UnixMilli())
	e.ResetArmAfterBoot()
	if e.armedAt() != 0 {
		t.Error("重启后该把挂着的关机准备作废")
	}
	if e.ShutdownState() != "等条件满足" {
		t.Error("作废后界面该回到「等条件满足」，实际：" + e.ShutdownState())
	}
}

func TestTriageKeepsTaskUnconsumedWhenGatewayDead(t *testing.T) {
	e := newTestEngine(t)
	id, _ := e.st.AddTask("正文", "w-same", "同模型窗口")
	e.triage(store.Task{ID: id}, win("w-same", "同模型窗口", "strata", true), errors.New("连接被中断"))
	q, _ := e.st.Queue()
	for _, tk := range q {
		if tk.ID == id {
			t.Error("标异常的任务不该还留在队列里被当成排队中")
		}
	}
	if es, _ := e.st.RecentErrors(10); len(es) != 1 || es[0].ID != id {
		t.Error("该进异常区，实际没进")
	}
}
