package store

import (
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	p := filepath.Join(t.TempDir(), "s.db")
	st, err := Open(p)
	if err != nil {
		t.Fatalf("开库失败: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// 一条任务能带多个附件；老任务只有 image_path 时要被当成唯一附件读出来。
func TestAttachPathsRoundTrip(t *testing.T) {
	st := openTestStore(t)

	id1, err := st.AddTask("看这两个图", "k1", "窗口1")
	if err != nil {
		t.Fatalf("加任务失败: %v", err)
	}
	paths := []string{`C:\tmp\a.png`, `C:\tmp\b.pdf`, `C:\tmp\c.png`}
	if err := st.SetAttachPaths(id1, paths); err != nil {
		t.Fatalf("记附件失败: %v", err)
	}

	id2, err := st.AddTask("老任务", "k2", "窗口2")
	if err != nil {
		t.Fatalf("加任务失败: %v", err)
	}
	if err := st.SetImagePath(id2, `C:\tmp\old.png`); err != nil {
		t.Fatalf("记图失败: %v", err)
	}

	got, err := st.Queue()
	if err != nil {
		t.Fatalf("读队列失败: %v", err)
	}
	byID := map[int]Task{}
	for _, tk := range got {
		byID[tk.ID] = tk
	}

	if a := byID[id1].AttachPaths; len(a) != 3 || a[1] != `C:\tmp\b.pdf` {
		t.Errorf("多附件没读全: %#v", a)
	}
	if a := byID[id2].AttachPaths; len(a) != 1 || a[0] != `C:\tmp\old.png` {
		t.Errorf("老任务的 image_path 没当附件读出来: %#v", a)
	}
	if byID[id2].ImagePath != `C:\tmp\old.png` {
		t.Errorf("image_path 本身丢了: %q", byID[id2].ImagePath)
	}
}

// 清空附件后不该还留着旧的。
func TestAttachPathsClear(t *testing.T) {
	st := openTestStore(t)
	id, err := st.AddTask("带图", "k", "窗口")
	if err != nil {
		t.Fatalf("加任务失败: %v", err)
	}
	if err := st.SetAttachPaths(id, []string{`C:\tmp\x.png`}); err != nil {
		t.Fatalf("记附件失败: %v", err)
	}
	if err := st.SetAttachPaths(id, nil); err != nil {
		t.Fatalf("清附件失败: %v", err)
	}
	q, err := st.Queue()
	if err != nil {
		t.Fatalf("读队列失败: %v", err)
	}
	for _, tk := range q {
		if tk.ID == id && len(tk.AttachPaths) != 0 {
			t.Errorf("清空后还剩: %#v", tk.AttachPaths)
		}
	}
}
