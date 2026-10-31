package engine

import "testing"

// 窗口清单「勾选会上下串行、点过去也变一下、下拉菜单也变」。
// 根因是按 updated_at 排（那个字段每有动静就变）。这里锁死两件事
// 1) 顺序只由「置顶 → 建窗时间 → 窗口键」决定，跟 updated_at 无关；
// 2) 勾选状态变了也不影响顺序。
func TestOrderWindowsStableAgainstActivity(t *testing.T) {
	base := []WindowInfo{
		{Key: "agent:main:feishu:group:g3", Label: "丙", CreatedAt: 300},
		{Key: "agent:main:main", Label: "甲", CreatedAt: 0}, // 无建窗时间 → 退到最后但仍稳定
		{Key: "agent:main:feishu:group:g1", Label: "乙", CreatedAt: 100},
		{Key: "agent:main:dashboard:x", Label: "丁", CreatedAt: 200},
	}

	want := []string{"agent:main:feishu:group:g1", "agent:main:dashboard:x", "agent:main:feishu:group:g3", "agent:main:main"}

	check := func(list []WindowInfo, tag string) {
		orderWindows(list)
		for i, k := range want {
			if list[i].Key != k {
				t.Fatalf("%s：第 %d 位应为 %s，实际 %s", tag, i+1, k, list[i].Key)
			}
		}
	}

	// 打乱 updated_at（模拟「有动静」）后顺序必须不变
	a := append([]WindowInfo{}, base...)
	for i := range a {
		a[i].UpdatedAt = int64(9999 - i)
	}
	check(a, "updated_at 变化后")

	// 勾选状态变化后顺序必须不变
	b := append([]WindowInfo{}, base...)
	for i := range b {
		b[i].Included = !b[i].Included
	}
	check(b, "勾选变化后")

	// 置顶的排最前（跟 OpenClaw 控制界面一致）：故意置顶建窗最晚的那个
	// 若排序正确，它应压过建窗时间跳到第一位。
	c := append([]WindowInfo{}, base...)
	for i := range c {
		if c[i].Key == "agent:main:feishu:group:g3" {
			c[i].Pinned = true
		}
	}
	orderWindows(c)
	if c[0].Key != "agent:main:feishu:group:g3" {
		t.Fatalf("置顶的应排最前，实际第一位是 %s", c[0].Key)
	}
}
