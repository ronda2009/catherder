// 导出 CSV（定稿 M4）：把任务记录导成表格，用户用 WPS/Excel 直接看。
// 带 UTF-8 BOM：不带的话 Windows 上的 Excel/WPS 会把中文显示成乱码（本机表格都是 WPS 开的）。
package store

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// ExportCSV 把任务表写到指定路径，返回写了几条。界面版与命令行版共用（默认上限 5000 条）。
func (s *Store) ExportCSV(path string) (int, error) {
	return s.ExportCSVRange(path, 5000, 0)
}

// ExportCSVRange 按范围导出（用户 2026-10-25：长期用条目会很多，得能选范围）。
// limit<=0 不限条数；since>0 只要该毫秒时刻之后提交的任务。
func (s *Store) ExportCSVRange(path string, limit int, since int64) (int, error) {
	if limit <= 0 {
		limit = 1000000
	}
	var tasks []Task
	var err error
	if since > 0 {
		tasks, err = s.queryTasks(`WHERE created_at >= ? ORDER BY id DESC LIMIT ?`, since, limit)
	} else {
		tasks, err = s.queryTasks(`ORDER BY id DESC LIMIT ?`, limit)
	}
	if err != nil {
		return 0, fmt.Errorf("读任务失败: %w", err)
	}
	if filepath.Ext(path) == "" {
		path += ".csv"
	}
	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("建文件失败: %w", err)
	}
	defer f.Close()
	if _, err := f.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil { // BOM：WPS/Excel 认中文
		return 0, err
	}
	w := csv.NewWriter(f)
	if err := w.Write([]string{etext("编号"), etext("目标窗口"), etext("状态"), etext("提交时间"), etext("派发时间"), etext("完成时间"), etext("回执"), etext("重试次数"), etext("失败原因"), etext("任务内容")}); err != nil {
		return 0, err
	}
	for _, t := range tasks {
		ack := ""
		switch {
		case t.Ack:
			ack = etext("有")
		case t.State == "done" || t.State == "done_unacked":
			ack = etext("没回")
		}
		if err := w.Write([]string{
			fmt.Sprintf("T-%03d", t.ID),
			orText(t.TargetLabel, t.TargetKey),
			StateText(t),
			fmtTimeMS(t.CreatedAt), fmtTimeMS(t.DispatchedAt), fmtTimeMS(t.DoneAt),
			ack, strconv.Itoa(t.Tries), t.Note, t.Body,
		}); err != nil {
			return 0, fmt.Errorf("写 CSV 失败: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return 0, err
	}
	s.Log("export", "导出 %d 条 → %s", len(tasks), path)
	return len(tasks), nil
}

// StateText 把内部状态翻成人话（导出与命令行共用一份，免得两处叫法不一致）。
func StateText(t Task) string {
	switch t.State {
	case "queued":
		return etext("排队中")
	case "dispatched":
		if t.Ack {
			return etext("已完成（有回执）")
		}
		return etext("已派发，等待完成")
	case "done":
		return etext("已完成")
	case "done_unacked":
		return etext("已完成（未收到回执）")
	case "error":
		return etext("派发失败")
	case "canceled":
		return etext("已删除")
	case "editing":
		return etext("正在修改")
	case "paused":
		return etext("已暂停")
	}
	return t.State
}

// ---- 导出表格的语言（C 阶段）----
// 表头与状态列跟着界面语言走；任务内容、窗口名、失败原因原文不动（那是 OpenClaw 那边的数据）。
// 命令行版不调 SetExportLang，所以永远是中文——跟以前一样。
var exportLang = "zh"

// SetExportLang 由界面版在启动与保存设置时调用；只认 en，其余＝中文。
func SetExportLang(lang string) {
	if lang == "en" {
		exportLang = "en"
	} else {
		exportLang = "zh"
	}
}

var exportEN = map[string]string{
	"编号": "ID", "目标窗口": "Target window", "状态": "State",
	"提交时间": "Submitted", "派发时间": "Dispatched", "完成时间": "Done",
	"回执": "Receipt", "重试次数": "Retries", "失败原因": "Error note", "任务内容": "Task text",
	"有": "yes", "没回": "no",
	"排队中": "Queued", "已派发，等待完成": "Dispatched, waiting",
	"已完成（有回执）": "Done (receipt)", "已完成（未收到回执）": "Done (no receipt)",
	"已完成": "Done", "派发失败": "Dispatch failed", "已删除": "Deleted", "已暂停": "Paused", "正在修改": "Editing",
}

func etext(zh string) string {
	if exportLang != "en" {
		return zh
	}
	if en, ok := exportEN[zh]; ok {
		return en
	}
	return zh
}

func orText(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func fmtTimeMS(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).In(time.Local).Format("2006-01-02 15:04:05")
}
