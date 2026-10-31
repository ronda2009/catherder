// 排队调序的测试（老板 2026-10-26 报「显示能调，发送还按原顺序」）。
// 前端「上移/下移」传给 Move 的 beforeID 之前算错一格（上移传上上条＝跳两格；
// 下移到队尾传 0＝被当队首）。这里按前端真实传参方式验一遍上下移动。
package store

import (
	"path/filepath"
	"testing"
)

func TestMoveUpDown(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("开测试库失败: %v", err)
	}
	defer st.Close()

	ids := []int{}
	for _, b := range []string{"A", "B", "C", "D"} {
		id, err := st.AddTask(b, "w-a", "窗口A")
		if err != nil {
			t.Fatalf("加任务失败: %v", err)
		}
		ids = append(ids, id)
	}
	order := func(tag string) []int {
		q, err := st.Queue()
		if err != nil {
			t.Fatalf("读队列失败: %v", err)
		}
		got := []int{}
		for _, x := range q {
			got = append(got, x.ID)
		}
		t.Logf("%s → %v", tag, got)
		return got
	}
	eq := func(a, b []int) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	if got := order("初始"); !eq(got, ids) {
		t.Fatalf("初始顺序不对: %v", got)
	}

	// 上移一位：把最后一条 D 挪到 C 之前（前端传「紧上面那条」的 id）。
	if err := st.Move(ids[3], ids[2]); err != nil {
		t.Fatalf("Move 失败: %v", err)
	}
	if got := order("D 上移一位"); !eq(got, []int{ids[0], ids[1], ids[3], ids[2]}) {
		t.Errorf("上移一位不对: 期望 [1 2 4 3]，实际 %v", got)
	}

	// 再上移一位：D 到 B 之前。
	if err := st.Move(ids[3], ids[1]); err != nil {
		t.Fatalf("Move 失败: %v", err)
	}
	if got := order("D 再上移一位"); !eq(got, []int{ids[0], ids[3], ids[1], ids[2]}) {
		t.Errorf("再上移一位不对: 期望 [1 4 2 3]，实际 %v", got)
	}

	// 移到队首：传 0。
	if err := st.Move(ids[3], 0); err != nil {
		t.Fatalf("Move 失败: %v", err)
	}
	if got := order("D 移到队首"); !eq(got, []int{ids[3], ids[0], ids[1], ids[2]}) {
		t.Errorf("移到队首不对: 期望 [4 1 2 3]，实际 %v", got)
	}

	// 下移到队尾：下面只剩一条，前端传 -1（传 0 会被当队首）。
	if err := st.Move(ids[3], -1); err != nil {
		t.Fatalf("Move 失败: %v", err)
	}
	if got := order("D 下移到队尾"); !eq(got, ids) {
		t.Errorf("下移到队尾不对: 期望 %v，实际 %v", ids, got)
	}

	// 中间下移一位：A 挪到 C 之前＝A 到第 2 位。
	if err := st.Move(ids[0], ids[2]); err != nil {
		t.Fatalf("Move 失败: %v", err)
	}
	if got := order("A 下移一位"); !eq(got, []int{ids[1], ids[0], ids[2], ids[3]}) {
		t.Errorf("下移一位不对: 期望 [2 1 3 4]，实际 %v", got)
	}
}
