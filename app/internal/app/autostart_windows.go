//go:build windows

package app

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// 开机自启只动「当前用户」的 Run 键（HKCU），写一个值、取消就删这个值：
// 不需要管理员权限，不动服务、不动机器级配置，随时可撤。
const (
	runKey    = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue  = "CatHerder"
	runLabel  = "工头派工台"
	runExeKey = "CatHerderExe"
)

// AutoStartInfo 给设置页显示：开没开、登记的是哪个程序、登记的是不是当前这份。
// 路径全部现读（注册表 + 当前进程），不写死——程序搬到哪台机器、放在哪个目录都照实显示。
type AutoStartInfo struct {
	Enabled bool   `json:"enabled"`
	ExePath string `json:"exePath"` // 当前正在运行的这份程序的路径
	RegPath string `json:"regPath"` // 开机启动登记处里记的路径（没开启则空）
	Stale   bool   `json:"stale"`   // 登记的与当前这份不是同一个位置（程序被搬过/改名过）
}

func (a *App) GetAutoStart() AutoStartInfo {
	self, _ := os.Executable()
	info := AutoStartInfo{ExePath: self}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return info
	}
	defer k.Close()
	v, _, err := k.GetStringValue(runValue)
	if err != nil || v == "" {
		return info
	}
	info.Enabled = true
	info.RegPath = strings.Trim(strings.Trim(v, `"`), `'`)
	info.Stale = !strings.EqualFold(info.RegPath, self)
	return info
}

// SetAutoStart 勾/取消开机自启。取消只删自己那一个值，不碰同键下别的东西。
func (a *App) SetAutoStart(on bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打不开开机启动登记处: %w", err)
	}
	defer k.Close()
	if !on {
		if err := k.DeleteValue(runValue); err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("取消失败: %w", err)
		}
		_ = k.DeleteValue(runExeKey)
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if err := k.SetStringValue(runValue, fmt.Sprintf(`"%s"`, self)); err != nil {
		return fmt.Errorf("写入失败: %w", err)
	}
	_ = k.SetStringValue(runExeKey, runLabel)
	return nil
}
