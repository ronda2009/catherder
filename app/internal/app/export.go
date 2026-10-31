// 导出 CSV 的界面入口：弹保存框选地方，真正写文件在 store.ExportCSV。
package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ExportCSV 弹窗让用户选存哪儿（默认名带日期）。按取消返回空字符串，不算失败。
// scope 是导出范围（用户 2026-10-25 问「条目多了怎么办」）：
// "50"/"200"/"7d"/"30d"/"all"；空或认不出＝最近 5000 条（旧行为）。
func (a *App) ExportCSV(scope string) (string, error) {
	// 文件名跟面板一个叫「工头派工台」（用户 2026-10-25 点头）。
	def := fmt.Sprintf(uiT("工头派工台记录")+"-%s.csv", time.Now().Format("200601-1504"))
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: def,
		Title:           uiT("导出任务记录 CSV"),
		Filters: []runtime.FileFilter{
			{DisplayName: uiT("CSV 表格 (*.csv)"), Pattern: "*.csv"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("弹保存框失败: %w", err)
	}
	if path == "" {
		return "", nil // 用户按了取消
	}
	limit, since, label := exportRange(scope)
	n, err := a.st.ExportCSVRange(path, limit, since)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("已导出%s %d 条 → %s", label, n, path), nil
}

// exportRange 把界面那个下拉的选项翻成「条数上限 + 起始时刻 + 给人看的说法」。
// 选项与默认值按用户 2026-10-25 定的：50 / 200 / 1000 / 1 天 / 3 天 / 7 天 / 30 天 / 全部，默认 50 条。
func exportRange(scope string) (int, int64, string) {
	dayMs := int64(24 * 3600 * 1000)
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "200":
		return 200, 0, "最近 200 条"
	case "1000":
		return 1000, 0, "最近 1000 条"
	case "1d":
		return 1000000, time.Now().UnixMilli() - dayMs, "最近 1 天"
	case "3d":
		return 1000000, time.Now().UnixMilli() - 3*dayMs, "最近 3 天"
	case "7d":
		return 1000000, time.Now().UnixMilli() - 7*dayMs, "最近 7 天"
	case "30d":
		return 1000000, time.Now().UnixMilli() - 30*dayMs, "最近 30 天"
	case "all":
		return 0, 0, "全部"
	default: // 50 与认不出的输入
		return 50, 0, "最近 50 条"
	}
}
