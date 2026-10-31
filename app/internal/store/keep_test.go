// 「已派发的」保留本轮的测试（用户 2026-10-25：完成的任务不许自动不见，
// 要一直留在「已派发的」里直到程序关掉；重开就是新的一轮）。
// 同时守住另一头：引擎算并发用的 Dispatched() 不能把已完成的也算进去。
package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDispatchedSinceKeepsDoneThisRun(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("开测试库失败: %v", err)
	}
	defer st.Close()

	// 上一轮：先派发再完成，并把派发/完成时刻直接改成 1 小时前（模拟「上次开机留下的」）。
	old, err := st.AddTask("上一轮的任务", "w-a", "窗口A")
	if err != nil {
		t.Fatalf("加任务失败: %v", err)
	}
	if err := st.MarkDispatched(old); err != nil {
		t.Fatalf("标已派发失败: %v", err)
	}
	if err := st.MarkDone(old, true); err != nil {
		t.Fatalf("标完成失败: %v", err)
	}
	longAgo := time.Now().Add(-time.Hour).UnixMilli()
	if _, err := st.db.Exec(`UPDATE tasks SET dispatched_at=?, done_at=? WHERE id=?`, longAgo, longAgo, old); err != nil {
		t.Fatalf("改时刻失败: %v", err)
	}

	boot := time.Now().Add(-time.Minute).UnixMilli() // 假装程序一分钟前刚重启

	// 本轮：一条仍在执行、一条已派发并完成。
	runA, _ := st.AddTask("本轮执行中", "w-b", "窗口B")
	runB, _ := st.AddTask("本轮已完成", "w-c", "窗口C")
	_ = st.MarkDispatched(runA)
	_ = st.MarkDispatched(runB)
	_ = st.MarkDone(runB, true)

	got, err := st.DispatchedSince(boot)
	if err != nil {
		t.Fatalf("查本轮已派发失败: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("本轮该留 2 条（执行中 + 已完成），实际 %d 条", len(got))
	}
	for _, tk := range got {
		if tk.ID == old {
			t.Error("上一轮完成的任务不该出现在本轮")
		}
	}

	// 引擎那边：只数真在跑的，已完成的不能算进并发。
	live, err := st.Dispatched()
	if err != nil {
		t.Fatalf("查执行中失败: %v", err)
	}
	if len(live) != 1 || live[0].ID != runA {
		t.Errorf("引擎该只看到 1 条在跑（T-%03d），实际 %+v", runA, live)
	}
}

// 空结果必须是空数组不是 nil：nil 到界面就是 null，取 .length 会报错
// （用户 2026-10-25 看到的「Cannot read properties of null (reading 'length')」）。
func TestQueriesReturnEmptyArrayNotNull(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("开测试库失败: %v", err)
	}
	defer st.Close()

	type check struct {
		name string
		f    func() ([]string, error)
	}
	for _, c := range []check{{"日志", func() ([]string, error) { return st.RecentEvents(10) }}} {
		v, err := c.f()
		if err != nil {
			t.Fatalf("%s 查询失败: %v", c.name, err)
		}
		if v == nil {
			t.Errorf("%s 空结果返回了 nil，界面取 .length 会崩", c.name)
		}
	}
	if q, err := st.Queue(); err != nil || q == nil {
		t.Errorf("队列空时应返回空数组，实际 %v / %v", q, err)
	}
	if d, err := st.Dispatched(); err != nil || d == nil {
		t.Errorf("执行中空时应返回空数组，实际 %v / %v", d, err)
	}
	if d, err := st.DispatchedSince(1); err != nil || d == nil {
		t.Errorf("本轮已派发空时应返回空数组，实际 %v / %v", d, err)
	}
	if e, err := st.RecentErrors(10); err != nil || e == nil {
		t.Errorf("异常空时应返回空数组，实际 %v / %v", e, err)
	}
	if p, err := st.Included(); err != nil || p == nil {
		t.Errorf("勾选窗口空时应返回空数组，实际 %v / %v", p, err)
	}
}
