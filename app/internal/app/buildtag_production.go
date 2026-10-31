//go:build production

// 带 production 标记编译时才是 true（界面版必须带这个标记，否则 wails.Run 会崩）。
package app

const BuiltProduction = true
