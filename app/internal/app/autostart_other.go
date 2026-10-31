//go:build !windows

package app

// 非 Windows 平台未适配开机自启（发布目标为 Windows）。

type AutoStartInfo struct {
	Enabled bool   `json:"enabled"`
	ExePath string `json:"exePath"`
}

func (a *App) GetAutoStart() AutoStartInfo { return AutoStartInfo{} }

func (a *App) SetAutoStart(bool) error { return nil }
