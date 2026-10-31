//go:build !windows

// 托盘只在 Windows 上有意义（本机就是 Windows）。别的平台上：不起托盘，点右上角叉照常退出。
package app

func startTrayImpl(a *App, iconBytes []byte) error { return nil }

// trayIconPath 非 Windows 不落临时图标文件。
func trayIconPath() string { return "" }

// trayAvailable 托盘到底起来没有。
func trayAvailable() bool { return false }

func trayHideWindow(a *App) {}

func trayQuitNow() {}

// trayRefreshTip 非 Windows 没托盘，悬停文字也就没得刷。
func trayRefreshTip() {}
