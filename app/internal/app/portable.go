// 换机器可移植性（定稿 §9a、§12）：通道/网关/服务全靠探测，不写死本机值。
// 探测不到任何聊天通道时给用户两个选择：「不用通知，只用界面」（默认，不烦人）／「我去配置，重试」。
package app

import (
	"os"

	"catherder/internal/openclaw"
)

type Probe struct {
	ConfigFound bool     `json:"configFound"`
	ConfigPath  string   `json:"configPath"`
	Channels    []string `json:"channels"`
	NeedChoice  bool     `json:"needChoice"` // 一个通道都没探测到、又没表过态 → 界面该弹那个选择框
	NoNotify    bool     `json:"noNotify"`   // 用户已选「不用通知」
	HasStrata   bool     `json:"hasStrata"`  // 可探测到模型服务状态（探测不到也可正常运行，仅少一个闸门）
}

// Probe 每次开界面、每次刷新都重算：换机器、改了 OpenClaw 配置都能立刻反映。
func (a *App) Probe() Probe {
	p := Probe{ConfigPath: a.cfg.Path}
	if a.cfg.Path != "" {
		if _, err := os.Stat(a.cfg.Path); err == nil {
			p.ConfigFound = true
		}
	}
	p.Channels = a.cfg.EnabledChannels()
	p.NoNotify = a.flagOn("notify_off", false)
	p.NeedChoice = len(p.Channels) == 0 && !p.NoNotify && !a.flagOn("notify_choice_made", false)
	_, known := a.eng.ServiceBusyForStatus()
	p.HasStrata = known
	return p
}

// ChooseNoNotify 用户选「不用通知，只用界面」：日志照记，设置里随时可再开。
func (a *App) ChooseNoNotify() error {
	a.st.SetSetting("notify_off", "1")
	a.st.SetSetting("notify_choice_made", "1")
	a.st.SetSetting("notify_channel", "")
	a.st.Log("notify", "这台机器没探测到聊天通道，用户选了「不用通知，只用界面」")
	return nil
}

// RetryProbe 用户选「我去配置，重试」：清掉选择标记，下一轮重新探测。
func (a *App) RetryProbe() (Probe, error) {
	a.st.SetSetting("notify_choice_made", "")
	a.st.SetSetting("notify_off", "")
	// 配置可能是刚改的：重读一遍再报（换机器时用户可能刚把 openclaw.json 放好）。
	if a.cfg.Path != "" {
		if c, err := openclaw.LoadConfig(a.cfg.Path); err == nil {
			a.cfg = c
		}
	}
	return a.Probe(), nil
}
