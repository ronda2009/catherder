// 界面语言 B 阶段：Go 侧只有这几处前端翻不了——任务栏托盘（菜单/悬停/气泡）与
// 系统弹的文件选择框、另存为框。其余给界面看的中文都经 feed/提示文字送到前端
// 前端 i18n.js 的词典会翻，这里不管（尤其 attach.go 的「发不出去」前端要拿它做判断
// 在 Go 这边翻了判断就失效）。
// 语言从设置里来（SaveSettings 调 SetUILang）；默认中文＝老行为不变。
package app

import "sync"

var (
	langMu   sync.RWMutex
	uiLangGo = "zh"
)

// uiT 按当前界面语言取文字；中文界面原样返回，词典没有的也原样返回。
func uiT(zh string) string {
	langMu.RLock()
	lang := uiLangGo
	langMu.RUnlock()
	if lang != "en" {
		return zh
	}
	if en, ok := dictEN[zh]; ok {
		return en
	}
	return zh
}

// SetUILang 由启动时与 SaveSettings 调用；顺带把已挂上的任务栏悬停文字重发一遍。
func SetUILang(lang string) {
	langMu.Lock()
	if lang == "en" {
		uiLangGo = "en"
	} else {
		uiLangGo = "zh"
	}
	langMu.Unlock()
	trayRefreshTip()
}

// 对照表：只登记托盘与系统对话框的文字。
var dictEN = map[string]string{
	// ---- 任务栏托盘 ----
	"｜左键叫回主界面，右键菜单":            " | left-click brings the window back, right-click opens the menu",
	"已在任务栏图标区常驻：左键叫回主界面，右键出菜单": "Living in the tray now: left-click brings the window back, right-click opens the menu",
	"已收到任务栏图标里，随时叫我：双击图标就回来。":  "I am in the tray now — double-click the icon to bring me back.",
	"打开主界面":  "Open main window",
	"打开「派工」": "Open Dispatch",
	"打开「窗口」": "Open Windows",
	"打开「日志」": "Open Log",
	"打开「设置」": "Open Settings",
	"恢复自动派工": "Resume auto dispatch",
	"暂停自动派工": "Pause auto dispatch",
	"退出":     "Quit",
	"托盘起不来：建消息窗失败（点右上角叉会直接退出，不会藏窗口）": "Tray unavailable: could not create the message window (closing the window quits instead of hiding it)",
	"托盘图标读不出来（窗口与功能照常，只是任务栏里没图标）":    "Tray icon could not be loaded (windows and features work fine, there is just no tray icon)",
	"托盘起不来：":       "Tray unavailable: ",
	"（点右上角叉会直接退出）": " (closing the window quits)",

	// ---- 系统文件选择框 / 另存为框 ----
	"选要随任务发出的文件（可多选）": "Choose files to send with the task (multiple allowed)",
	"另存为":            "Save As",
	"PNG 图片":         "PNG image",
	"截图-":            "screenshot-",
	"工头派工台记录":        "foreman-log",
	"导出任务记录 CSV":     "Export task log CSV",
	"CSV 表格 (*.csv)": "CSV spreadsheet (*.csv)",
}
