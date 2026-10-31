//go:build !production

// 没带 production 标记时的默认值：开界面会在 wails.Run 里直接崩（实测踩过，甩一堆 Go 栈），
// 所以入口程序靠这个值提前给一句人话用法。
package app

const BuiltProduction = false
