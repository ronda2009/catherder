// 「取消异常任务」（dismiss）的测试：不再派发、不算删除、能重试、能删、不拦派发。
package store

import (
	"path/filepath"
	"testing"
)

func openDismissStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("开测试库失败: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func addErrorRow(t *testing.T, st *Store, body string) int {
	t.Helper()
	id, err := st.AddTask(body, "w-a", "窗口A")
	if err != nil {
		t.Fatalf("加任务失败: %v", err)
	}
	if err := st.MarkError(id, "模型服务不通"); err != nil {
		t.Fatalf("标异常失败: %v", err)
	}
	return id
}

func TestDismissKeepsTask(t *testing.T) {
	st := openDismissStore(t)
	id := addErrorRow(t, st, "甲")
	if err := st.Dismiss(id); err != nil {
		t.Fatalf("取消失败: %v", err)
	}
	es, err := st.RecentErrors(50)
	if err != nil {
		t.Fatalf("读异常失败: %v", err)
	}
	if len(es) != 1 || es[0].State != "dismissed" {
		t.Fatalf("取消后该留在异常列表里（state=dismissed），实际 %+v", es)
	}
	q, _ := st.Queue()
	if len(q) != 0 {
		t.Errorf("取消的任务不该回到队列，实际 %d 条", len(q))
	}
	c, _ := st.Counts()
	if c["error"] != 0 {
		t.Errorf("取消后不该再算待处理异常，实际 %d", c["error"])
	}
	// 取消过的还能重试回队列
	if err := st.RetryTask(id); err != nil {
		t.Fatalf("重试已取消的任务失败: %v", err)
	}
	if q, _ := st.Queue(); len(q) != 1 {
		t.Errorf("重试后该回队列，实际 %d 条", len(q))
	}
}

func TestDismissAllClearsHalt(t *testing.T) {
	st := openDismissStore(t)
	addErrorRow(t, st, "甲")
	addErrorRow(t, st, "乙")
	if err := st.SetSetting("halt", "2 条任务均为异常"); err != nil {
		t.Fatalf("写停机标记失败: %v", err)
	}
	if err := st.DismissAllErrors(); err != nil {
		t.Fatalf("批量取消失败: %v", err)
	}
	if h, _ := st.GetSetting("halt"); h != "" {
		t.Errorf("批量取消后停机标记该当场清掉，实际还留着: %q", h)
	}
	es, _ := st.RecentErrors(50)
	if len(es) != 2 {
		t.Errorf("两条都该留在异常列表里，实际 %d 条", len(es))
	}
	// 单条取消：还剩一条未处理时，停机标记不该清
	id1 := addErrorRow(t, st, "丙")
	id2 := addErrorRow(t, st, "丁")
	if err := st.SetSetting("halt", "还有异常"); err != nil {
		t.Fatalf("写停机标记失败: %v", err)
	}
	if err := st.Dismiss(id1); err != nil {
		t.Fatalf("取消失败: %v", err)
	}
	if h, _ := st.GetSetting("halt"); h == "" {
		t.Error("还有未处理异常时停机标记不该清")
	}
	// 最后一条也取消了 → 当场清停机标记（不等下一轮检查）
	if err := st.Dismiss(id2); err != nil {
		t.Fatalf("取消失败: %v", err)
	}
	if h, _ := st.GetSetting("halt"); h != "" {
		t.Errorf("最后一条异常取消后停机标记该清掉，实际还留着: %q", h)
	}
	// 已取消的还能删除
	if err := st.Cancel(id1); err != nil {
		t.Fatalf("删除已取消的任务失败: %v", err)
	}
	if es, _ := st.RecentErrors(50); len(es) != 3 {
		t.Errorf("删除后异常列表该剩 3 条，实际 %d 条", len(es))
	}
}
