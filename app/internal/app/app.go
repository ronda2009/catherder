// 界面与核心之间的胶水层：把 engine/store 的能力包成 Wails 能调的方法。
// 这里不放业务逻辑（都在 internal/engine、internal/store），只做三件事
// 给前端取数据、执行前端指令、把后台循环的动静推给前端。
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"catherder/internal/engine"
	"catherder/internal/openclaw"
	"catherder/internal/store"
)

type App struct {
	ctx      context.Context
	cfg      *openclaw.Config
	st       *store.Store
	eng      *engine.Engine
	mu       sync.Mutex
	stop     chan struct{}
	running  bool
	interval int
	nextTick int64 // 下一次检查的时刻（毫秒）；给界面算「距下次检查还剩几秒」
	bootMs   int64 // 本次开机的时刻；「已派发的」只留本轮
	startTab string
	trayIcon []byte
	imgDir   string // 截图目录
}

// Deps 由入口程序传进来（入口负责解析命令行参数、决定用哪个库文件）。
type Deps struct {
	Cfg         *openclaw.Config
	Store       *store.Store
	Engine      *engine.Engine
	IntervalSec int
	StartTab    string // 启动时先打开哪个页（调试用）
	TrayIcon    []byte // 右下角任务栏图标（.ico 内容，入口从 exe 里读）
	ImageDir    string // 截图存放目录（跟自己的库放一起， portable 时跟着走）
}

func New(d Deps) *App {
	iv := d.IntervalSec
	if iv < 15 {
		iv = 60
	}
	return &App{cfg: d.Cfg, st: d.Store, eng: d.Engine, interval: iv, startTab: d.StartTab, trayIcon: d.TrayIcon, imgDir: d.ImageDir, bootMs: time.Now().UnixMilli()}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	// 上次没按「改好了」就关掉的条先推回排队（只在界面版启动时做，命令行不开这个头）。
	a.st.RecoverEditing()
	// 界面语言先按存好的设置定下来（托盘菜单/系统对话框在 Go 这边，得先知道语言）。
	if v, _ := a.st.GetSetting("ui_lang"); v == "en" {
		SetUILang("en")
		store.SetExportLang("en")
	}
	a.eng.SetOut(func(s string) { a.push(s) })
	a.push(uiT("工头派工台已启动"))
	// 重启前挂着的「准备关机」先作废：Windows 那边那条待关机已随开机没了
	// 标记留着会让界面一开机就挂倒计时，下一轮还会再下一次关机。
	// 放在 SetOut/push 之后，这句才会进界面日志。
	a.eng.ResetArmAfterBoot()
	a.StartLoop()
	a.StartTray(a.trayIcon)
	// 文件拖进窗口（拖到输入区）：当成选了这几个文件，列在待发附件里。
	runtime.OnFileDrop(ctx, func(_, _ int, paths []string) {
		if len(paths) == 0 {
			return
		}
		runtime.EventsEmit(ctx, "drop-files", paths)
	})
}

func (a *App) Shutdown(context.Context) { a.StopLoop() }

// ---- 右下角任务栏常驻----

// trayTitle 是右下角图标悬停与气泡显示的名字（入口 --label 给）
// 两份并存时靠它区分正式在跑的与开发调试用的。
var trayTitle = "工头派工台 CatHerder"

// SetTrayTitle 在起托盘之前设好名字（main.go 解析完参数就调）。
func SetTrayTitle(s string) {
	if s != "" {
		trayTitle = s
	}
}

// StartTray 起托盘图标。图标读不到时不起托盘：那时点右上角叉就该直接退出
// 不能把用户的窗口藏没了又没地方叫回来。
func (a *App) StartTray(iconBytes []byte) {
	if err := startTrayImpl(a, iconBytes); err != nil {
		a.push("托盘起不来：" + err.Error() + "（点右上角叉会直接退出）")
	}
}

// TrayReady 告诉入口：托盘真的起来了，点叉才可以「收到托盘」。
func (a *App) TrayReady() bool { return trayAvailable() }

// TrayOnClose 读设置：点右上角叉是「收到托盘继续干活」还是「直接退出」。
// 默认收到托盘；设置里关掉就回到老行为。
func (a *App) TrayOnClose() bool { return a.flagOn("tray_on_close", true) }

// HideMainWindow 藏主窗（托盘还在，双击图标能叫回来）。
func (a *App) HideMainWindow() {
	if a.ctx != nil {
		runtime.WindowHide(a.ctx)
	}
}

// ShowMainWindow 从托盘叫回主界面：显示 + 取消最小化 + 拉到前台。
func (a *App) ShowMainWindow() {
	if a.ctx == nil {
		return
	}
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

// OpenTab 托盘菜单直接打开某一页。
// 先叫回窗口，再发个事件让界面自己切页（界面不在时事件也不会丢：叫窗后 boot 会跑，但页靠这条）。
func (a *App) OpenTab(tab string) {
	a.ShowMainWindow()
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "tab", tab)
}

// QuitApp 托盘菜单「退出」：先撤托盘图标，再让 Wails 收摊（走 OnBeforeClose 之外的真退出）。
func (a *App) QuitApp() {
	quitting = true
	trayQuitNow()
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// StartTab 告诉前端启动时先打开哪个页（调试时能直接看到设置/日志页）。
func (a *App) StartTab() string { return a.startTab }

// BuildStamp 是编译时通过 -ldflags -X 注进来的时刻。
// 界面上显示它，使用者一眼就能确认跑的是哪一版（避免「改了却看不出变化」）。
var BuildStamp string

// BuildVersion 也是编译时注进来的——正式版要能报出自己是哪一版。
var BuildVersion string

func (a *App) BuildStamp() string { return BuildStamp }

func (a *App) BuildVersion() string { return BuildVersion }

// Debug 把前端自测信息写进日志，查界面问题时用（--tab debug 触发）。
func (a *App) Debug(msg string) { a.push("[前端] " + msg) }

// push 把消息同时送界面与事件日志。界面版是 windowsgui（无控制台）
// 光靠 fmt.Println 看不见，所以再落一份进库里，用 `catherder.exe log` 就能读。
// 自检信息靠这个拿，比裁图猜可靠。

func (a *App) push(msg string) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "feed", msg)
	}
	fmt.Println(msg)
	if a.st != nil {
		a.st.Log("debug", "%s", msg)
	}
}

// quitting 为真表示「真要走」（托盘菜单点退出、或设置里关了收到托盘）
// 那时 OnBeforeClose 不能再拦，否则关不掉。
var quitting bool

// ShouldHideOnClose 点右上角叉时该不该「收到托盘继续干活」。
func (a *App) ShouldHideOnClose() bool {
	return !quitting && a.TrayReady() && a.TrayOnClose()
}

// HideToTray 收到右下角任务栏：藏主窗 + 右下角弹一下「已最小化，随时叫我」。
func (a *App) HideToTray() { trayHideWindow(a) }

// TrayEnabled 给设置页读：叉掉窗口时是收到托盘还是直接退出。
func (a *App) TrayEnabled() bool { return a.TrayOnClose() }

// ---- 循环控制 ----

func (a *App) StartLoop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running {
		return
	}
	a.running = true
	a.stop = make(chan struct{})
	a.setNextTick()
	go func() {
		t := time.NewTicker(time.Duration(a.interval) * time.Second)
		defer t.Stop()
		for {
			select {
			case <-a.stop:
				return
			case <-t.C:
				if err := a.eng.Tick(false); err != nil {
					a.push("本次检查出错：" + err.Error())
				}
				a.rearmNextTick() // 本轮跑完重新计时，界面那个倒计时才准
				runtime.EventsEmit(a.ctx, "refresh", true)
			}
		}
	}()
	a.push(fmt.Sprintf("已开始跟踪窗口，每 %d 秒检查一次", a.interval))
}

// setNextTick 记下下一次检查的时刻（毫秒）。
// ❗调用方必须已经持有 a.mu：Go 的互斥锁不可重入，StartLoop 拿着锁再取一次会永久卡死
// 不持锁的场合用 rearmNextTick。
func (a *App) setNextTick() {
	a.nextTick = time.Now().Add(time.Duration(a.interval) * time.Second).UnixMilli()
}

// rearmNextTick 自己取锁再重新计时，给不持锁的调用方（定时循环、手动检查）。
func (a *App) rearmNextTick() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.setNextTick()
}

func (a *App) StopLoop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.running {
		return
	}
	close(a.stop)
	a.running = false
	a.push("已暂停自动派发")
}

func (a *App) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

// 手动触发一次检查（界面「立即检查一次」）：跑完把结果说清。
// 原来什么都不回，界面只能写「已检查一次」，看不出这一轮到底派没派。
// 出错也当结果文字返回（不报 error）：Wails 把 error 抛成异常，界面就看不到本轮结果了。
func (a *App) TickNow() string {
	beforeQ, _ := a.st.Queue()
	beforeRun, _ := a.st.Dispatched() // 只数真在跑的，才能算出「这一轮完成了几条」
	err := a.eng.Tick(false)
	a.rearmNextTick() // 查过一轮，倒计时重新计时才准
	if err != nil {
		return "本次检查出错：" + err.Error()
	}
	afterQ, _ := a.st.Queue()
	afterRun, _ := a.st.Dispatched()
	sent := len(beforeQ) - len(afterQ)
	if sent < 0 {
		sent = 0
	}
	finished := len(beforeRun) - len(afterRun)
	if finished < 0 {
		finished = 0
	}
	msg := fmt.Sprintf("已检查一轮：派出去 %d 条", sent)
	if finished > 0 {
		msg += fmt.Sprintf("，完成 %d 条", finished)
	}
	if halt, _ := a.st.GetSetting("halt"); halt != "" {
		msg += "（派发已停止：" + halt + "）"
	} else if paused, _ := a.st.GetSetting("paused"); paused != "" {
		msg += "（派发已暂停：" + paused + "）"
	}
	return msg
}

// ---- 数据给前端 ----

func (a *App) Windows() ([]engine.WindowInfo, error) { return a.eng.Snapshot() }

func (a *App) Queue() ([]store.Task, error) { return a.st.Queue() }

// Dispatched 给界面「已派发的」：本轮开机以来派出去的都留着，已完成的也不抹。
// 引擎算并发不看这里，看 store.Dispatched()（只数真在跑的），两者不混。
func (a *App) Dispatched() ([]store.Task, error) { return a.st.DispatchedSince(a.bootMs) }

// Errors 供界面「异常任务」区使用：含原因，配「重试 / 删除」按钮。
func (a *App) Errors() ([]store.Task, error) { return a.st.RecentErrors(50) }

func (a *App) RecentLog(n int) ([]string, error) { return a.st.RecentEvents(n) }

// ListConversations 给设置页：把「收件地址」那一长串变成可选名单。
func (a *App) ListConversations(channel string) ([]engine.ConversationInfo, error) {
	return a.eng.ListConversations(channel)
}

type Overview struct {
	Running  bool   `json:"running"`
	Interval int    `json:"interval"`
	Paused   string `json:"paused"`
	Halt     string `json:"halt"`
	Notify   string `json:"notify"`
	Parallel int    `json:"parallel"`
	Service  string `json:"service"`
	// ServiceKind / ServiceWhy：认成了哪种服务（llama.cpp / ollama / llama-swap / 状态接口）、凭什么说它忙。
	// 顶栏那一栏只放结论，这两个给悬停与日志用——不然「服务空闲」四个字看不出它到底认了什么。
	ServiceKind string `json:"serviceKind"`
	ServiceWhy  string `json:"serviceWhy"`
	QueueLen    int    `json:"queueLen"`
	DispatchN   int    `json:"dispatchN"`
	DoneN       int    `json:"doneN"` // 顶栏「完成 N」（状态栏要有已完成的计数）
	ErrorN      int    `json:"errorN"`
	Shutdown    string `json:"shutdown"`
	// NextSec：距下一次检查还剩几秒。
	// 只能给全局这一个节拍：引擎是每隔 interval 秒跑一轮，不是每条任务各自倒计时。
	NextSec int `json:"nextSec"`
}

func (a *App) Overview() Overview {
	o := Overview{Running: a.IsRunning(), Interval: a.interval}
	a.mu.Lock()
	nx := a.nextTick
	a.mu.Unlock()
	if a.running && nx > 0 {
		s := int((nx - time.Now().UnixMilli()) / 1000)
		if s < 0 {
			s = 0
		}
		if s > a.interval {
			s = a.interval
		}
		o.NextSec = s
	}
	o.Paused, _ = a.st.GetSetting("paused")
	ch, _ := a.st.GetSetting("notify_channel")
	if ch == "" {
		o.Notify = "不发通知（只记日志）"
	} else {
		o.Notify = ch
	}
	o.Parallel = a.settingInt("max_parallel", 1)
	if d := a.eng.ServiceBusyDetail(); d.Known {
		if d.Busy {
			o.Service = "模型服务繁忙"
		} else {
			o.Service = "模型服务空闲"
		}
		o.ServiceKind, o.ServiceWhy = d.Kind, d.Why
	} else {
		o.Service = "这台机器探不到服务状态，只看窗口"
		o.ServiceWhy = "状态地址上 /status、/slots、/api/ps、/running 都没答（探不到就只看窗口状态）"
	}
	if q, err := a.st.Queue(); err == nil {
		o.QueueLen = len(q)
	}
	if d, err := a.st.Dispatched(); err == nil {
		o.DispatchN = len(d)
	}
	o.Halt, _ = a.st.GetSetting("halt")
	if c, err := a.st.Counts(); err == nil {
		o.ErrorN = c["error"]
	}
	// 顶栏「完成 N」与界面「已完成」区必须同一个口径
	// 只算本轮判完工的（收了回执、还是只凭窗口空闲判完，都算）。
	if d, err := a.st.DispatchedSince(a.bootMs); err == nil {
		n := 0
		for _, t := range d {
			if t.State != "dispatched" {
				n++
			}
		}
		o.DoneN = n
	}
	o.Shutdown = a.eng.ShutdownState()
	return o
}

func (a *App) settingInt(key string, def int) int {
	v, _ := a.st.GetSetting(key)
	if v == "" {
		return def
	}
	var n int
	fmt.Sscanf(v, "%d", &n)
	if n <= 0 {
		return def
	}
	return n
}

// ---- 前端指令 ----

func (a *App) AddTask(targetKey, body string) (int, error) {
	label := ""
	for _, p := range mustPref(a.st) {
		if p.Key == targetKey {
			label = p.Label
		}
	}
	return a.st.AddTask(body, targetKey, label)
}

func (a *App) CancelTask(id int) error { return a.st.Cancel(id) }

// StartEdit 把排队任务转成「正在改」（队列里点「编辑」）：这条先不派发，改完按「改好了」才参与派发。
func (a *App) StartEdit(id int) error { return a.st.StartEditing(id) }

// SaveEdit 队列里就地改完：存正文与发送窗口，这条回到排队（原位置）。
// body 传空＝内容没改（只换了发送窗口）。
func (a *App) SaveEdit(id int, targetKey, body string) error {
	label := ""
	for _, p := range mustPref(a.st) {
		if p.Key == targetKey {
			label = p.Label
		}
	}
	return a.st.FinishEditing(id, body, targetKey, label)
}

// CancelEdit 放弃这次修改（按原样继续排队）。
func (a *App) CancelEdit(id int) error { return a.st.CancelEditing(id) }

// RetryTask 将标记为异常的任务重新排队（修复后点击「重试」）。
func (a *App) RetryTask(id int) error { return a.st.RetryTask(id) }

// ClearHalt 清除「全部异常已停止派发」标记，恢复派发。
func (a *App) ClearHalt() error { return a.st.ClearHalt() }

// PendingConfirm 给界面轮询同模型并发确认；没有要问的返回 nil。
func (a *App) PendingConfirm() *engine.ConfirmRequest { return a.eng.PendingConfirm() }

// AnswerConfirm 处理对并发确认的答复：accept＝仍然派发；mute＝""|"today"|"run"。
func (a *App) AnswerConfirm(accept bool, mute string) error { return a.eng.AnswerConfirm(accept, mute) }

// CancelShutdown 界面「取消关机」按钮：补一条 shutdown /a。
func (a *App) CancelShutdown() { a.eng.CancelShutdown() }

func (a *App) MoveTask(id, beforeID int) error { return a.st.Move(id, beforeID) }

func (a *App) SetIncluded(key string, on bool) error { return a.st.SetIncluded(key, on) }

// Settings 给设置页用：一次把常用项都取出来。
type Settings struct {
	NotifyChannel string `json:"notifyChannel"`
	NotifyTarget  string `json:"notifyTarget"`
	MaxParallel   int    `json:"maxParallel"`
	RecheckSec    int    `json:"recheckSec"`
	RetryDelaySec int    `json:"retryDelaySec"`
	AppendTail    bool   `json:"appendTail"`
	GateService   bool   `json:"gateService"`
	GateGpu       bool   `json:"gateGpu"`
	GpuPercent    int    `json:"gpuPercent"`
	// ServiceStatusUrl：服务忙闲去哪个地址问（留空＝http://127.0.0.1:8080）。
	// 以前只能走命令行 set，别人机器上换了端口就只能猜；现在设置页里能自己改。
	ServiceStatusUrl string   `json:"serviceStatusUrl"`
	AutoInclude      bool     `json:"autoInclude"`
	IntervalSec      int      `json:"intervalSec"`
	TrayOnClose      bool     `json:"trayOnClose"`
	AutoShutdown     bool     `json:"autoShutdown"`
	ShutdownHours    string   `json:"shutdownHours"`
	MuteMode         string   `json:"muteMode"`
	Lang             string   `json:"lang"`
	Channels         []string `json:"channels"`
}

func (a *App) GetSettings() Settings {
	s := Settings{
		MaxParallel:   a.settingInt("max_parallel", 1),
		RecheckSec:    a.settingInt("recheck_seconds", 60),
		RetryDelaySec: a.settingInt("retry_delay_seconds", 10),
		IntervalSec:   a.settingInt("check_interval_seconds", 60),
		Channels:      a.cfg.EnabledChannels(),
	}
	s.NotifyChannel, _ = a.st.GetSetting("notify_channel")
	s.NotifyTarget, _ = a.st.GetSetting("notify_target")
	s.AppendTail = a.flagOn("append_tail", true)
	s.GateService = a.flagOn("gate_service", true)
	s.GateGpu = a.flagOn("gate_gpu", true)
	s.GpuPercent = a.settingInt("gpu_busy_percent", 30)
	s.ServiceStatusUrl, _ = a.st.GetSetting("service_status_url")
	s.AutoInclude = a.flagOn("auto_include_new", false)
	s.TrayOnClose = a.flagOn("tray_on_close", true)
	s.AutoShutdown = a.flagOn("auto_shutdown", false)
	s.ShutdownHours, _ = a.st.GetSetting("shutdown_hours")
	s.MuteMode, _ = a.st.GetSetting("mute_mode")
	s.Lang, _ = a.st.GetSetting("ui_lang")
	return s
}

func (a *App) SaveSettings(s Settings) error {
	set := func(k, v string) { a.st.SetSetting(k, v) }
	set("notify_channel", s.NotifyChannel)
	set("notify_target", s.NotifyTarget)
	// 用户在设置里又选了通道，说明不要「不用通知」了，把那个标记清掉。
	if s.NotifyChannel != "" {
		set("notify_off", "")
		set("notify_choice_made", "1")
	}
	// 上限放到 5。
	set("max_parallel", fmt.Sprint(clamp(s.MaxParallel, 1, 5)))
	set("recheck_seconds", fmt.Sprint(clamp(s.RecheckSec, 5, 600)))
	set("retry_delay_seconds", fmt.Sprint(clamp(s.RetryDelaySec, 1, 300)))
	set("append_tail", boolStr(s.AppendTail))
	set("gate_service", boolStr(s.GateService))
	set("gate_gpu", boolStr(s.GateGpu))
	set("gpu_busy_percent", fmt.Sprint(clamp(s.GpuPercent, 5, 95)))
	set("service_status_url", strings.TrimSpace(s.ServiceStatusUrl))
	set("auto_include_new", boolStr(s.AutoInclude))
	set("tray_on_close", boolStr(s.TrayOnClose))
	iv := clamp(s.IntervalSec, 15, 600)
	set("check_interval_seconds", fmt.Sprint(iv))
	set("auto_shutdown", boolStr(s.AutoShutdown))
	set("shutdown_hours", strings.TrimSpace(s.ShutdownHours))
	// 界面语言（A 阶段：只存设置，前端自己换界面）。
	if s.Lang == "en" {
		set("ui_lang", "en")
	} else {
		set("ui_lang", "")
	}
	// 托盘菜单/悬停、系统对话框这些文字在 Go 这边，语言一变要立刻跟着变；导出表格的列名同理。
	SetUILang(s.Lang)
	store.SetExportLang(s.Lang)
	if s.MuteMode != "today" && s.MuteMode != "run" {
		set("mute_mode", "")
		set("mute_date", "")
	}
	a.mu.Lock()
	a.interval = iv
	a.mu.Unlock()
	return nil
}

func (a *App) flagOn(key string, def bool) bool {
	v, _ := a.st.GetSetting(key)
	if v == "" {
		return def
	}
	return v == "1"
}

// ExportCSVNow 给设置页「导出 CSV」用：不弹保存框，直接写到「文档」目录（命令行与测试用）。
func (a *App) ExportCSVNow() (string, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		dir = "."
	}
	path := filepath.Join(dir, "Documents", fmt.Sprintf("派工台记录-%s.csv", time.Now().Format("200601-1504")))
	n, err := a.st.ExportCSV(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("已导出 %d 条 → %s", n, path), nil
}

// NotifyTest 让设置页「试发一条」按钮验证通道通不通。
func (a *App) NotifyTest(text string) error {
	return a.eng.NotifyNow(text)
}

func mustPref(st *store.Store) []store.WindowPref {
	p, _ := st.Included()
	return p
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
