//go:build !windows

package app

// 非 Windows 平台暂不做开机自启（本机是 Windows，发布目标也是 Windows）。

type AutoStartInfo struct {
	Enabled bool   `json:"enabled"`
	ExePath string `json:"exePath"`
}

func (a *App) GetAutoStart() AutoStartInfo { return AutoStartInfo{} }

func (a *App) SetAutoStart(bool) error { return nil }
