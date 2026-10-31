// 收件地址名单的实测（用户 2026-10-25：点「刷新地址」没出来东西、切通道下面也不变）。
// 连真网关跑一次 conversations.list，把结果打进测试日志；连不上就跳过（不拖垮离线测试）。
// 跑法：go test -count=1 -run TestListConversationsLive -v ./internal/app
package app

import (
	"os"
	"path/filepath"
	"testing"

	"catherder/internal/engine"
	"catherder/internal/openclaw"
	"catherder/internal/store"
)

func TestListConversationsLive(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "openclaw.json")
	body, err := os.ReadFile(`C:\Users\Administrator\.openclaw\openclaw.json`)
	if err != nil {
		t.Skipf("读不到本机配置，跳过: %v", err)
	}
	if err := os.WriteFile(cfgPath, body, 0o644); err != nil {
		t.Fatalf("写配置失败: %v", err)
	}
	cfg, err := openclaw.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("读配置失败: %v", err)
	}
	st, err := store.Open(filepath.Join(dir, "c.sqlite"))
	if err != nil {
		t.Fatalf("开库失败: %v", err)
	}
	defer st.Close()

	a := New(Deps{Cfg: cfg, Store: st, Engine: engine.New(cfg, st, ""), IntervalSec: 60})
	for _, ch := range append(cfg.EnabledChannels(), "") {
		list, err := a.ListConversations(ch)
		if err != nil {
			t.Logf("通道[%s] 查询失败: %v", ch, err)
			continue
		}
		t.Logf("通道[%s] 共 %d 条", ch, len(list))
		for i, c := range list {
			if i >= 6 {
				t.Logf("  …其余 %d 条", len(list)-6)
				break
			}
			t.Logf("  kind=%s label=%s target=%s ref=%s", c.Kind, c.Label, c.Target, c.Ref)
		}
	}
}
