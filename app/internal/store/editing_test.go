package store

import "path/filepath"
import "testing"

// TestEditingFlow 验「队列里就地编辑」
// 正在改的那条不派发、也不挡后面的任务；改完回到原来的位置；上方新建任务的内容不受影响。
func TestEditingFlow(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "e.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	a, _ := st.AddTask("第一条", "k1", "窗口一")
	b, _ := st.AddTask("第二条", "k1", "窗口一")
	c, _ := st.AddTask("第三条", "k1", "窗口一")

	// 点「编辑」：转成正在改，内容还在库里（没丢），状态不是 queued。
	if err := st.StartEditing(b); err != nil {
		t.Fatal(err)
	}
	q, err := st.Queue()
	if err != nil {
		t.Fatal(err)
	}
	if len(q) != 3 {
		t.Fatalf("正在改的任务也该留在排队列表里，得到 %d 条", len(q))
	}
	if q[1].ID != b || q[1].State != "editing" || q[1].Body != "第二条" || !q[1].Editing {
		t.Fatalf("第二条应转成正在改且内容还在，得到 %+v", q[1])
	}

	// 改完：正文与发送窗口都换，位置不变（还在第二格）。
	if err := st.FinishEditing(b, "第二条改好了", "k2", "窗口二"); err != nil {
		t.Fatal(err)
	}
	q, _ = st.Queue()
	if q[1].ID != b || q[1].State != "queued" || q[1].Editing {
		t.Fatalf("改完应回到第二格并结束编辑，得到 %+v", q[1])
	}
	if q[1].Body != "第二条改好了" || q[1].TargetKey != "k2" || q[1].TargetLabel != "窗口二" {
		t.Fatalf("正文/窗口没存进去，得到 %+v", q[1])
	}

	// 只换窗口、内容不改（界面传空正文）。
	if err := st.StartEditing(a); err != nil {
		t.Fatal(err)
	}
	if err := st.FinishEditing(a, "", "k3", "窗口三"); err != nil {
		t.Fatal(err)
	}
	all, _ := st.AllTasks(50)
	for _, tk := range all {
		if tk.ID == a && (tk.Body != "第一条" || tk.TargetKey != "k3") {
			t.Fatalf("只换窗口时正文被改了或窗口没换: %+v", tk)
		}
	}

	// 放弃修改：库里内容还是原样。
	if err := st.StartEditing(c); err != nil {
		t.Fatal(err)
	}
	if err := st.CancelEditing(c); err != nil {
		t.Fatal(err)
	}
	all, _ = st.AllTasks(50)
	for _, tk := range all {
		if tk.ID == c && (tk.Body != "第三条" || tk.State != "queued" || tk.Editing) {
			t.Fatalf("放弃修改后应变回原样并排队: %+v", tk)
		}
	}

	// 已派发的不能转成正在改（不能把正在跑的抢回来）。
	if err := st.MarkDispatched(a); err != nil {
		t.Fatal(err)
	}
	if err := st.StartEditing(a); err == nil {
		t.Fatal("已派发的任务不该能转成正在改")
	}
}
