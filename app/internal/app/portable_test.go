// 换机器可移植性测试：模拟「未配置飞书、未安装 Strata」的机器，确认仍能扫描窗口、正常运行，
// 并且通知探测不到时会给出「不用通知」的选择而不是报错。全程不连网关、不占模型。
package app

import (
	"os"
	"path/filepath"
	"testing"

	"catherder/internal/engine"
	"catherder/internal/openclaw"
	"catherder/internal/store"
)

func newTestApp(t *testing.T, channels string) *App {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "openclaw.json")
	body := `{"gateway":{"port":18789,"auth":{"mode":"token","token":"x"}},"channels":` + channels +
		`,"models":{"providers":{}}}`
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("写假配置失败: %v", err)
	}
	cfg, err := openclaw.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("读假配置失败: %v", err)
	}
	st, err := store.Open(filepath.Join(dir, "c.sqlite"))
	if err != nil {
		t.Fatalf("开测试库失败: %v", err)
	}
	t.Cleanup(st.Close)
	// 服务状态地址指到一个不存在的端口：模拟「这台机器没装 Strata」。
	st.SetSetting("service_status_url", "http://127.0.0.1:1")
	return New(Deps{Cfg: cfg, Store: st, Engine: engine.New(cfg, st, ""), IntervalSec: 60})
}

func TestProbeNoChannelAsksForChoice(t *testing.T) {
	a := newTestApp(t, `{}`)
	p := a.Probe()
	if !p.ConfigFound {
		t.Error("配置文件在，该报找到了")
	}
	if len(p.Channels) != 0 {
		t.Errorf("这台机器没配通道，实际报出 %v", p.Channels)
	}
	if !p.NeedChoice {
		t.Error("一个通道都没探测到又没表过态，该弹「不用通知 / 我去配置」的选择")
	}
	if p.HasStrata {
		t.Error("服务状态探测不到时不应报告存在 Strata（仅少一个闸门，不影响运行）")
	}
	// 选「不用通知」：不再弹，日志照记，设置里随时可再开
	if err := a.ChooseNoNotify(); err != nil {
		t.Fatalf("选不用通知失败: %v", err)
	}
	p2 := a.Probe()
	if p2.NeedChoice {
		t.Error("表过态了就不该再弹")
	}
	if !p2.NoNotify {
		t.Error("该记住用户选了不用通知")
	}
	if ch, _ := a.st.GetSetting("notify_channel"); ch != "" {
		t.Error("选择不发送通知后应清空通道，避免每次检查空跑")
	}
	// 设置里重新选通道 → 「不用通知」标记自动清掉
	if err := a.SaveSettings(Settings{NotifyChannel: "telegram", MaxParallel: 1, RecheckSec: 30, RetryDelaySec: 10, IntervalSec: 60}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if a.flagOn("notify_off", false) {
		t.Error("重新选了通道，「不用通知」标记该自动清掉")
	}
}

func TestProbeWithChannelNoAsk(t *testing.T) {
	a := newTestApp(t, `{"feishu":{"enabled":true}}`)
	p := a.Probe()
	if len(p.Channels) != 1 || p.Channels[0] != "feishu" {
		t.Fatalf("该探测到 feishu，实际 %v", p.Channels)
	}
	if p.NeedChoice {
		t.Error("探测到通道就不该弹选择框")
	}
}

func TestProbeRetryRereadsConfig(t *testing.T) {
	a := newTestApp(t, `{}`)
	if err := a.ChooseNoNotify(); err != nil {
		t.Fatalf("选不用通知失败: %v", err)
	}
	// 用户去把配置改好了（加了通道），点「我去配置，重试」该重读配置并探测到
	body := `{"gateway":{"port":18789,"auth":{"mode":"token","token":"x"}},"channels":{"telegram":{}},"models":{"providers":{}}}`
	if err := os.WriteFile(a.cfg.Path, []byte(body), 0o644); err != nil {
		t.Fatalf("改假配置失败: %v", err)
	}
	p, err := a.RetryProbe()
	if err != nil {
		t.Fatalf("重试探测失败: %v", err)
	}
	if len(p.Channels) != 1 || p.Channels[0] != "telegram" {
		t.Errorf("重试后该探测到新配的 telegram，实际 %v", p.Channels)
	}
	if p.NeedChoice {
		t.Error("探测到了就不该再弹")
	}
}
