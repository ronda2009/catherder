// 导出 CSV 的测试：BOM、行数、状态要翻成人话、正文里的逗号换行不能被写坏。
package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportCSV(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("开测试库失败: %v", err)
	}
	defer st.Close()
	if _, err := st.AddTask("第一行\n第二行，带逗号", "w-a", "窗口A"); err != nil {
		t.Fatalf("加任务失败: %v", err)
	}
	id2, _ := st.AddTask("第二条", "w-b", "窗口B")
	if err := st.MarkError(id2, "模型服务不通"); err != nil {
		t.Fatalf("标异常失败: %v", err)
	}

	path := filepath.Join(t.TempDir(), "out.csv")
	n, err := st.ExportCSV(path)
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	if n != 2 {
		t.Errorf("该导 2 条，实际 %d", n)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读导出文件失败: %v", err)
	}
	if len(b) < 3 || b[0] != 0xEF || b[1] != 0xBB || b[2] != 0xBF {
		t.Error("缺 UTF-8 BOM：WPS/Excel 开中文会乱码（本机表格都是 WPS 开的）")
	}
	text := string(b[3:])
	for _, want := range []string{"编号", "失败原因", "T-001", "窗口A", "派发失败", "模型服务不通"} {
		if !strings.Contains(text, want) {
			t.Errorf("导出内容里找不到 %q", want)
		}
	}
	if !strings.Contains(text, `"第一行`+"\n"+`第二行，带逗号"`) {
		t.Error("正文里的换行与逗号没被引号包住，表格会把一条拆成两行")
	}
	if strings.Contains(text, "queued") || strings.Contains(text, "dispatched") {
		t.Error("状态该翻成人话（排队中/已派出），不该把内部英文漏进表格")
	}
}

func TestExportCSVAddsExtension(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("开测试库失败: %v", err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "noext")
	if _, err := st.ExportCSV(path); err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	if _, err := os.Stat(path + ".csv"); err != nil {
		t.Error("没写扩展名时该自动补 .csv，否则 WPS 不认")
	}
}
