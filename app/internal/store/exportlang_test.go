// 导出表格的语言（C 阶段）：英文界面下表头与状态列变英文
// 任务内容/窗口名/失败原因保持原文；命令行版不调 SetExportLang，所以永远是中文。
package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportCSVLang(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("开测试库失败: %v", err)
	}
	defer st.Close()
	id1, _ := st.AddTask("任务正文，带逗号", "w-a", "窗口A")
	if err := st.MarkError(id1, "模型服务不通"); err != nil {
		t.Fatalf("标异常失败: %v", err)
	}

	// 默认（命令行版走这条）：中文表头 + 中文状态。
	p1 := filepath.Join(t.TempDir(), "zh.csv")
	if _, err := st.ExportCSV(p1); err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	b1, _ := os.ReadFile(p1)
	s1 := string(b1)
	if !strings.Contains(s1, "编号,目标窗口,状态") {
		t.Errorf("中文表头不对: %s", firstLine(s1))
	}
	if !strings.Contains(s1, "派发失败") {
		t.Errorf("中文状态不对: %s", firstLine(s1))
	}

	// 英文界面：表头与状态变英文，正文/窗口名/原因保持原文。
	SetExportLang("en")
	defer SetExportLang("zh")
	p2 := filepath.Join(t.TempDir(), "en.csv")
	if _, err := st.ExportCSV(p2); err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	b2, _ := os.ReadFile(p2)
	s2 := string(b2)
	if !strings.Contains(s2, "ID,Target window,State") {
		t.Errorf("英文表头不对: %s", firstLine(s2))
	}
	if !strings.Contains(s2, "Dispatch failed") {
		t.Errorf("英文状态不对: %s", firstLine(s2))
	}
	if !strings.Contains(s2, "任务正文，带逗号") {
		t.Errorf("任务正文被改了: %s", firstLine(s2))
	}
	if !strings.Contains(s2, "窗口A") {
		t.Errorf("窗口名被改了: %s", firstLine(s2))
	}
	if !strings.Contains(s2, "模型服务不通") {
		t.Errorf("失败原因被改了: %s", firstLine(s2))
	}
	t.Logf("英文表头行: %s", firstLine(s2))
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
