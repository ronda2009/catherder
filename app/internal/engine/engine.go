// 派工台核心：一轮检查 = 扫窗口 → 收尾已派发的 → 派发下一条。
// 完成判定与失败分诊按《设计定稿.md》§4/§7；本程序全程不执行模型回合。
package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"catherder/internal/openclaw"
	"catherder/internal/store"
)

const (
	maxReplyChars = 1200 // 与现有飞书转发脚本一致：超长截断并注明全文在窗口
	freshInputMs  = 5000 // 定稿 §11：目标窗口 5 秒内刚有新输入 → 本次顺延
	notifyTries   = 3
)

type Engine struct {
	cfg       *openclaw.Config
	st        *store.Store
	dbPath    string
	ws        *openclaw.Client
	notifyRef string
	out       func(string)
	// 显卡忙闲：连续几轮超阈值才算忙，所以要在两轮之间记住读数。
	gpuAt    time.Time
	gpuPct   int
	gpuHot   int
	gpuKnown bool
}

func New(cfg *openclaw.Config, st *store.Store, dbPath string) *Engine {
	return &Engine{cfg: cfg, st: st, dbPath: dbPath, out: func(s string) { fmt.Println(s) }}
}

func (e *Engine) SetOut(fn func(string)) { e.out = fn }

func (e *Engine) settingInt(key string, def int) int {
	v, _ := e.st.GetSetting(key)
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

func (e *Engine) flagOn(key string, def bool) bool {
	v, _ := e.st.GetSetting(key)
	if v == "" {
		return def
	}
	return v == "1"
}

func (e *Engine) recheckSec() int    { return e.settingInt("recheck_seconds", 60) }
func (e *Engine) capWindows() int    { return e.settingInt("max_parallel", 1) }
func (e *Engine) retryDelaySec() int { return e.settingInt("retry_delay_seconds", 10) }

// DBPath 暴露给命令行版复用（避免两处各写一遍库路径）。
func (e *Engine) DBPath() string { return e.dbPath }

// ServiceBusyForStatus 给 status 命令与界面用，与内部闸门同一逻辑。
func (e *Engine) ServiceBusyForStatus() (bool, bool) { return e.serviceBusy() }

// GpuBusyForStatus 给 status 命令与界面用（回：忙？、读得到？、原因）。
func (e *Engine) GpuBusyForStatus() (bool, bool, string) { return e.gpuBusy() }

// SettingInterval 给入口程序取「多久查一轮」，与闸门同一套读取。
func (e *Engine) SettingInterval() int { return e.settingInt("check_interval_seconds", 60) }

// Tick 跑一轮检查。dryRun 只读库、不改状态、不发消息。
func (e *Engine) Tick(dryRun bool) error {
	db, err := openclaw.OpenDB(e.dbPath)
	if err != nil {
		return fmt.Errorf("读会话库失败: %w", err)
	}
	defer db.Close()

	wins, included, err := e.scan(db)
	if err != nil {
		return err
	}
	e.finishDispatched(db, wins, dryRun)
	if err := e.dispatch(wins, included, dryRun); err != nil {
		return err
	}
	if !dryRun {
		e.checkHalt()
		e.tickShutdown(included, wins, dryRun)
	}
	return nil
}

// checkHalt 队列里一条能派的都没有了（全部异常）→ 停止派发等待处理。
// 只要仍有一条排队或执行中的任务就不停止，避免误停。
func (e *Engine) checkHalt() {
	c, err := e.st.Counts()
	if err != nil {
		return
	}
	halt, _ := e.st.GetSetting("halt")
	// 正在就地修改的那条也算「还没完」：别在它改的时候判定队列空去关机。
	stuck := c["queued"] == 0 && c["dispatched"] == 0 && c["editing"] == 0 && c["error"] > 0
	switch {
	case stuck && halt == "":
		msg := fmt.Sprintf("%d 条任务均为异常", c["error"])
		if e.st.SetSetting("halt", msg); err == nil {
			e.st.Log("halt", "队列中 %d 条任务均为异常，已停止派发，等待处理", c["error"])
			e.out("已停止派发：" + msg)
			e.notify(fmt.Sprintf("⚠️ 工头派工台：队列中 %d 条任务均为异常，已停止派发。修复后请在界面点击「重试」或「继续派发」。", c["error"]))
		}
	case !stuck && halt != "":
		_ = e.st.ClearHalt()
		e.out("异常已处理，自动恢复派发")
	}
}

// WindowInfo 是给界面看的窗口快照（含勾选状态与模型在线情况）。
type WindowInfo struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Status    string `json:"status"`
	Idle      bool   `json:"idle"`
	Pending   int    `json:"pending"`
	Included  bool   `json:"included"`
	Model     string `json:"model"`
	Provider  string `json:"provider"`
	ModelKind string `json:"modelKind"` // 本地在线 / 本地离线 / 云端 / 未知
	UpdatedAt int64  `json:"updatedAt"`
	CreatedAt int64  `json:"createdAt"` // 定固定顺序用（不随活动变）
	Pinned    bool   `json:"pinned"`    // 用户在 OpenClaw 里置顶过的，排最前
}

// orderWindows 定「固定顺序」：置顶优先 → 建窗时间（早的在前）→ 键名。
// 不用 updated_at 排：那个字段每有动静（包括勾选、刷新、派活）就变
// 列表就会上下串行。
// 建窗时间和键名都不随活动变 → 只要窗口没新建/删除，位置就钉死。
// 置顶跟 OpenClaw 控制界面一致：用户在那边置顶了，这边也跟着排最前。
func orderWindows(list []WindowInfo) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Pinned != list[j].Pinned {
			return list[i].Pinned
		}
		if list[i].CreatedAt != list[j].CreatedAt {
			if list[i].CreatedAt == 0 {
				return false // 没建窗时间的退到后面，但仍稳定
			}
			if list[j].CreatedAt == 0 {
				return true
			}
			return list[i].CreatedAt < list[j].CreatedAt
		}
		return list[i].Key < list[j].Key
	})
}

// Snapshot 刷新并返回可派工窗口清单，界面与命令行共用（编号顺序一致）。
func (e *Engine) Snapshot() ([]WindowInfo, error) {
	db, err := openclaw.OpenDB(e.dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	wins, included, err := e.scan(db)
	if err != nil {
		return nil, err
	}
	out := make([]WindowInfo, 0, len(wins))
	for _, w := range wins {
		out = append(out, WindowInfo{
			Key: w.Key, Label: w.Label, Status: w.Status, Idle: w.Idle(), Pending: w.Pending,
			Included: included[w.Key], Model: w.Model, Provider: w.Provider,
			ModelKind: e.providerKind(w.Provider), UpdatedAt: w.UpdatedAt,
			Pinned: w.PinnedAt > 0, CreatedAt: w.CreatedAt,
		})
	}
	orderWindows(out)
	return out, nil
}

// ConversationInfo 给设置页用：把「那一长串收件地址」变成能选的名单。
type ConversationInfo struct {
	Label  string `json:"label"`
	Target string `json:"target"`
	Ref    string `json:"ref"`
	Kind   string `json:"kind"`
}

// ListConversations 向网关要一份会话清单（可按通道筛）。连不上/查不到就报错
// 由界面提示用户（搜不到怎么提示：界面里给「没查到，可继续手填」）。
func (e *Engine) ListConversations(channel string) ([]ConversationInfo, error) {
	ws, err := e.ensureWS()
	if err != nil {
		return nil, err
	}
	// limit 最大只能 100。
	pl, err := ws.Call("conversations.list", map[string]any{"agentId": "main", "limit": 100})
	if err != nil {
		// 连接断了先重连再查一次。
		ws2, rerr := e.reconnect()
		if rerr != nil {
			return nil, err
		}
		pl, err = ws2.Call("conversations.list", map[string]any{"agentId": "main", "limit": 100})
		if err != nil {
			return nil, err
		}
	}
	list, _ := pl["conversations"].([]any)
	var out []ConversationInfo
	seen := map[string]bool{} // 同一个地址只留一条（网关会把同一会话的历史记录都列出来，名单会重复）
	for _, it := range list {
		m, _ := it.(map[string]any)
		ch, _ := m["channel"].(string)
		if channel != "" && ch != channel {
			continue
		}
		ref, _ := m["conversationRef"].(string)
		if ref == "" {
			continue
		}
		tg, _ := m["target"].(string)
		if tg == "" || seen[tg] {
			continue
		}
		seen[tg] = true
		lb, _ := m["label"].(string)
		kd, _ := m["kind"].(string)
		// 网关给的标签带通道前缀（如「feishu:g-墨云」），去掉前缀与 g-/d- 才好看。
		if base := strings.TrimPrefix(lb, ch+":"); base != lb {
			lb = strings.TrimPrefix(strings.TrimPrefix(base, "g-"), "d-")
		}
		if lb == "" {
			lb = tg
		}
		out = append(out, ConversationInfo{Label: lb, Target: tg, Ref: ref, Kind: kd})
	}
	return out, nil
}

// providerKind 把「本地/云端 + 通不通」合成一个给人看的词。
func (e *Engine) providerKind(provider string) string {
	if provider == "" {
		return "未知"
	}
	local, _ := e.cfg.ProviderIsLocal(provider)
	if !local {
		return "云端"
	}
	if alive, known := e.providerAlive(provider); known {
		if alive {
			return "本地在线"
		}
		return "本地离线"
	}
	return "本地"
}

// scan 刷新可见窗口清单。心跳/定时任务/子代理窗口记为“已排除”，永不纳入。
func (e *Engine) scan(db *openclaw.DB) (map[string]openclaw.Window, map[string]bool, error) {
	list, err := db.Windows()
	if err != nil {
		return nil, nil, err
	}
	out := map[string]openclaw.Window{}
	seen := []string{}
	for _, w := range list {
		e.st.UpsertWindow(w.Key, w.Label, w.Status, w.Model, w.Provider, w.Schedulable())
		if !w.Schedulable() || w.Archived {
			continue // 归档的窗口不纳入也不派（和 OpenClaw 控制界面一致）
		}
		out[w.Key] = w
		seen = append(seen, w.Key)
	}
	e.st.AutoIncludeNew(seen)
	prefs, err := e.st.Included()
	if err != nil {
		return nil, nil, err
	}
	inc := map[string]bool{}
	for _, p := range prefs {
		inc[p.Key] = true
	}
	return out, inc, nil
}

// finishDispatched 判定完成：窗口状态已结束 + 无待处理输入 + 延时复核。
// 模型空闲不等于任务完成，因此这里只看窗口，不看服务。
func (e *Engine) finishDispatched(db *openclaw.DB, wins map[string]openclaw.Window, dryRun bool) {
	ds, err := e.st.Dispatched()
	if err != nil {
		return
	}
	now := time.Now().UnixMilli()
	for _, t := range ds {
		w, ok := wins[t.TargetKey]
		if !ok {
			if !dryRun {
				e.st.MarkError(t.ID, "目标窗口不存在或已归档")
			}
			e.notify(fmt.Sprintf("⚠️ 任务 T-%03d 的目标窗口已不存在（被归档或删除），已标记为异常", t.ID))
			continue
		}
		if !w.Idle() || w.Pending > 0 {
			e.st.ClearIdle(t.ID) // 又忙了，复核重新计时
			continue
		}
		if t.IdleSince == 0 {
			e.st.MarkIdle(t.ID, now)
			e.out(fmt.Sprintf("T-%03d 目标窗口已空闲，开始 %d 秒复核", t.ID, e.recheckSec()))
			continue
		}
		if now-t.IdleSince < int64(e.recheckSec())*1000 {
			continue
		}
		e.finishOne(db, t, w, dryRun)
	}
}

func (e *Engine) finishOne(db *openclaw.DB, t store.Task, w openclaw.Window, dryRun bool) {
	ack := e.hasAck(db, w.ID, t)
	if dryRun {
		e.out(fmt.Sprintf("[dry] T-%03d 判定完成（完成回执=%v）", t.ID, ack))
		return
	}
	e.st.MarkDone(t.ID, ack)
	if ack {
		e.out(fmt.Sprintf("T-%03d 已完成并收到回执 ✅", t.ID))
	} else {
		// 完成回执只是加强证据：窗口状态说已完成就照常推进，卡片标灰
		e.out(fmt.Sprintf("T-%03d 已完成（依据窗口状态判定，未收到回执）", t.ID))
	}
	e.forward(db, t, w)
}

func (e *Engine) hasAck(db *openclaw.DB, sessionID string, t store.Task) bool {
	for _, needle := range []string{
		fmt.Sprintf("✅T-%03d", t.ID),
		fmt.Sprintf("DONE T-%03d", t.ID), // 英文界面下的回执（定稿 §8）
	} {
		if ok, err := db.ContainsSince(sessionID, t.DispatchedAt, needle); err == nil && ok {
			return true
		}
	}
	return false
}

// forward 任务结果先转发到聊天通道，再派发下一条。
// 没设通知通道时直接算「已送达」：的判定——没设飞书通知，窗口回完就算完成。
func (e *Engine) forward(db *openclaw.DB, t store.Task, w openclaw.Window) {
	if ch, _ := e.st.GetSetting("notify_channel"); ch == "" {
		_ = e.st.MarkNotified(t.ID)
		return
	}
	text, err := db.LastAssistantText(w.ID)
	if err != nil {
		e.st.Log("forward", "T-%03d 读回复失败: %v", t.ID, err)
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		e.st.Log("forward", "T-%03d 已完成但读取不到回复内容（可能位于上下文压缩事件中）", t.ID)
		return
	}
	ch, _ := e.st.GetSetting("notify_channel")
	if ch != "" && strings.Contains(w.Key, ":"+ch+":") {
		e.out(fmt.Sprintf("T-%03d 结果已由 %s 通道送达，不重复转发", t.ID, ch))
		_ = e.st.MarkNotified(t.ID) // 目标窗口本来就在通知通道里，结果已经送到那边了
		return
	}
	body, tail := text, ""
	if r := []rune(body); len(r) > maxReplyChars {
		body = string(r[:maxReplyChars])
		tail = "\n……（全文在窗口里）"
	}
	msg := fmt.Sprintf("📤【%s %s】\n%s%s", displayName(w), time.Now().Format("15:04"), body, tail)
	if e.notify(msg) {
		_ = e.st.MarkNotified(t.ID) // 结果已送达通知通道；界面「已完成」区靠这个判断
	}
}

// dispatch 从队首往下扫，跳过目标窗口还忙的，派第一条能派的。
func (e *Engine) dispatch(wins map[string]openclaw.Window, included map[string]bool, dryRun bool) error {
	ws, err := e.ensureWS()
	if err != nil {
		if !dryRun {
			e.pause("gateway: " + err.Error())
		}
		e.out("网关连接失败，本次不派发：" + err.Error())
		return nil
	}
	paused, _ := e.st.GetSetting("paused")
	if strings.HasPrefix(paused, "gateway:") {
		e.st.SetSetting("paused", "") // 连上了就是恢复了
		e.out("网关已恢复，自动恢复派发")
	} else if paused != "" {
		e.out("派发已暂停（" + paused + "），本次跳过")
		return nil
	}
	if halt, _ := e.st.GetSetting("halt"); halt != "" {
		e.out("派发已停止（" + halt + "），等待处理")
		return nil
	}

	ds, err := e.st.Dispatched()
	if err != nil {
		return err
	}
	live := len(ds)
	if live >= e.capWindows() {
		return nil
	}
	// 本地模型服务忙不忙：只用来挡「用本地模型的窗口」，云端窗口不受这个判断。
	var busyGate bool
	if e.flagOn("gate_service", true) {
		if busy, known := e.serviceBusy(); known && busy {
			busyGate = true
		}
	}
	// 显卡忙不忙：服务自己没报忙、但显卡还在动的时候也别派。
	// 「绘图员h 被误判闲置」就是这种：别的活占着显卡，服务端口却不报忙。
	var gpuBlock string
	if e.flagOn("gate_gpu", true) {
		if busy, known, why := e.gpuBusy(); known && busy {
			gpuBlock = why
		}
	}

	q, err := e.st.Queue()
	if err != nil {
		return err
	}
	blocked := "" // 排队却没派出去的原因
	for _, t := range q {
		if live >= e.capWindows() {
			blocked = fmt.Sprintf("同时已派满 %d 个窗口", e.capWindows())
			break
		}
		if gpuBlock != "" {
			// 显卡是整机资源，不分哪个窗口；它忙就全体先等。
			blocked = gpuBlock
			break
		}
		if t.State == "editing" {
			// 正在改：先不派，后面的任务接着派。
			blocked = "有一条正在就地修改，先跳过它"
			continue
		}
		w, ok := wins[t.TargetKey]
		if !ok || !included[t.TargetKey] {
			blocked = "目标窗口不在派工范围（或已不存在）"
			continue
		}
		if busyGate {
			if busy, known := e.providerBusy(w.Provider); known && busy {
				blocked = fmt.Sprintf("窗口 %s 用的模型服务正忙，这条先排队", displayName(w))
				continue
			}
		}
		if !w.Idle() || w.Pending > 0 {
			blocked = fmt.Sprintf("窗口 %s 正忙（还有 %d 条没处理完）", displayName(w), w.Pending)
			continue
		}
		if time.Now().UnixMilli()-w.UpdatedAt < freshInputMs {
			blocked = fmt.Sprintf("窗口 %s 刚收到消息，本轮先避开", displayName(w))
			continue // 刚收到输入，状态可能尚未更新
		}
		if dryRun {
			e.out(fmt.Sprintf("[试运行] 将把 T-%03d 派发给 %s", t.ID, displayName(w)))
			live++
			continue
		}
		// 同模型并发：已有一条在执行、目标窗口使用同一模型服务 → 先请求确认。
		if live > 0 && e.capWindows() > 1 {
			switch e.confirmFor(t, w, ds, wins) {
			case confirmBlock:
				continue
			case confirmAsk:
				continue
			case confirmPass:
			}
		}
		sent, gatewayDead := e.sendOne(ws, t, w)
		if gatewayDead {
			return nil
		}
		if sent {
			live++
			blocked = ""
		}
	}
	if blocked != "" {
		// 试运行也说清为什么没派出去（命令行 `tick --dry` 看不到原因就只能干着急）。
		e.out("排队任务暂未派发：" + blocked)
	}
	return nil
}

// confirmFor 三种结果：放行 / 拦住本次不派 / 挂起等待确认。
func (e *Engine) confirmFor(t store.Task, w openclaw.Window, ds []store.Task, wins map[string]openclaw.Window) confirmResult {
	req := e.sameModelConflict(t, w, ds, wins)
	if req == nil {
		return confirmPass // 不同模型或云端探不到 → 不弹（定稿 §6）
	}
	if e.confirmMuted() {
		return confirmPass // 勾过不再提醒
	}
	if grant, skip := e.confirmDecision(t.ID); grant {
		return confirmPass
	} else if skip {
		return confirmBlock
	}
	if e.PendingConfirm() != nil {
		return confirmBlock // 已在等待其他确认，本次不派
	}
	e.setConfirmPending(*req)
	e.out("T-" + fmt.Sprintf("%03d", t.ID) + " 待派发，但目标窗口与执行中的任务使用同一模型服务，已挂起等待确认")
	return confirmAsk
}

// sendOne 派发一条；失败按 分诊：网关挂了不消耗任务，模型不通标异常跳过。
func (e *Engine) sendOne(ws *openclaw.Client, t store.Task, w openclaw.Window) (sent bool, gatewayDead bool) {
	msg := t.Body
	if e.flagOn("append_tail", true) {
		msg += fmt.Sprintf("\n\n——【工头派工台 T-%03d】完成后回一行：✅T-%03d", t.ID, t.ID)
	}
	// 带附件的任务：文件跟文字一起发（网关 sessions.send 的 attachments，base64）。
	// 文件读不到（被删了/移动了）不默默丢下：降级成只发文字并在日志里说清。
	send := func(c *openclaw.Client) error {
		paths := t.AttachPaths
		if len(paths) == 0 {
			return c.SendToWindow(t.TargetKey, msg)
		}
		files := make([]openclaw.Attach, 0, len(paths))
		for _, p := range paths {
			raw, rerr := os.ReadFile(p)
			if rerr != nil {
				e.st.Log("attach", "T-%03d 的附件 %s 读不到（%v），这个不发了", t.ID, filepath.Base(p), rerr)
				continue
			}
			mime, name := attachKind(p)
			files = append(files, openclaw.Attach{Mime: mime, Data: raw, Name: name})
		}
		if len(files) == 0 {
			return c.SendToWindow(t.TargetKey, msg)
		}
		return c.SendToWindowWithFiles(t.TargetKey, msg, files)
	}

	err := send(ws)
	if err != nil {
		time.Sleep(time.Duration(e.retryDelaySec()) * time.Second)
		ws2, rerr := e.reconnect()
		if rerr != nil {
			e.pause("gateway: " + rerr.Error())
			e.notify(fmt.Sprintf("⚠️ 工头派工台：网关连接失败，已暂停派发（任务 T-%03d 尚未发出）", t.ID))
			return false, true
		}
		e.ws = ws2
		if err2 := send(ws2); err2 != nil {
			e.triage(t, w, err2)
			return false, false
		}
		ws = ws2
	}
	if err := e.st.MarkDispatched(t.ID); err != nil {
		return false, false
	}
	e.out(fmt.Sprintf("T-%03d 已派发给 %s", t.ID, displayName(w)))
	_ = ws
	return true, false
}

func (e *Engine) triage(t store.Task, w openclaw.Window, cause error) {
	if alive, known := e.providerAlive(w.Provider); known && !alive {
		e.st.MarkError(t.ID, "模型服务不通: "+cause.Error())
		e.notify(fmt.Sprintf("⚠️ 任务 T-%03d 异常：窗口 %s 的模型服务（%s）不可用，已跳过并继续下一条",
			t.ID, displayName(w), w.Provider))
		return
	}
	e.st.MarkError(t.ID, cause.Error())
	e.notify(fmt.Sprintf("⚠️ 任务 T-%03d 派发失败，已标记为异常：%v", t.ID, cause))
}

func (e *Engine) pause(reason string) {
	e.st.SetSetting("paused", reason)
	e.st.Log("pause", "%s", reason)
}

// ensureWS 复用已有连接；断了自动重连（派发与转发共用这一条）。
func (e *Engine) ensureWS() (*openclaw.Client, error) {
	if e.ws != nil {
		return e.ws, nil
	}
	return e.reconnect()
}

func (e *Engine) reconnect() (*openclaw.Client, error) {
	if e.ws != nil {
		e.ws.Close()
		e.ws = nil
		e.notifyRef = ""
	}
	c, err := openclaw.Dial(e.cfg)
	if err != nil {
		return nil, err
	}
	e.ws = c
	return c, nil
}

// resolveNotifyRef 查通道地址。连接被掉时报的是“连接被中断”而不是“找不到通道”
// 不重连就会把整条通知丢掉，所以查失败先重连一次再查。
func (e *Engine) resolveNotifyRef(ws *openclaw.Client, ch string) (*openclaw.Client, string, error) {
	if e.notifyRef != "" {
		return ws, e.notifyRef, nil
	}
	tg, _ := e.st.GetSetting("notify_target")
	ref, err := ws.FindConversation("main", ch, tg)
	if err == nil && ref != "" {
		e.notifyRef = ref
		return ws, ref, nil
	}
	first := err
	if first == nil {
		first = fmt.Errorf("没查到匹配地址")
	}
	ws2, rerr := e.reconnect()
	if rerr != nil {
		return ws, "", fmt.Errorf("查通道地址失败(%v)，重连也失败(%v)", first, rerr)
	}
	ref, err = ws2.FindConversation("main", ch, tg)
	if err != nil || ref == "" {
		if err == nil {
			err = first
		}
		return ws2, "", fmt.Errorf("在 %s 里没找到目标地址: %v", ch, err)
	}
	e.notifyRef = ref
	return ws2, ref, nil
}

// notify 发通知；探测不到通道就只记日志。
// notify 把消息发到通知通道；发成功返回 true（界面「已完成」区靠这个判断结果送达没有）。
func (e *Engine) notify(text string) bool {
	ch, _ := e.st.GetSetting("notify_channel")
	if ch == "" {
		e.st.Log("notify", "未配置通知通道，已跳过发送")
		return false
	}
	ws, err := e.ensureWS()
	if err != nil {
		e.st.Log("notify", "网关不可用，通知未发: %v", err)
		return false
	}
	ws, ref, err := e.resolveNotifyRef(ws, ch)
	if err != nil {
		e.st.Log("notify", "找不到通道地址，通知未发: %v", err)
		return false
	}
	for i := 0; i < notifyTries; i++ {
		if err := ws.SendToConversation("main", ref, text); err == nil {
			return true
		}
		time.Sleep(2 * time.Second)
		if ws2, rerr := e.reconnect(); rerr == nil {
			ws = ws2
			if ws2, ref, rerr = e.resolveNotifyRef(ws2, ch); rerr != nil {
				e.st.Log("notify", "重连后找不到通道地址，通知未发: %v", rerr)
				return false
			}
		}
	}
	e.st.Log("notify", "发送失败（已重试 %d 次）", notifyTries)
	return false
}

// NotifyNow 给设置页「试发一条」用：立刻发一条，失败就把原因返回给调用方。
func (e *Engine) NotifyNow(text string) error {
	ch, _ := e.st.GetSetting("notify_channel")
	if ch == "" {
		return fmt.Errorf("未选择通知通道")
	}
	ws, err := e.ensureWS()
	if err != nil {
		return fmt.Errorf("连网关失败: %w", err)
	}
	ws, ref, err := e.resolveNotifyRef(ws, ch)
	if err != nil {
		return err
	}
	if err := ws.SendToConversation("main", ref, text); err == nil {
		return nil
	}
	// 连接可能是旧的：重连一次再发。
	ws2, rerr := e.reconnect()
	if rerr != nil {
		return err
	}
	ws2, ref2, rerr := e.resolveNotifyRef(ws2, ch)
	if rerr != nil {
		return err
	}
	return ws2.SendToConversation("main", ref2, text)
}

// serviceBusy 读模型服务的忙闲（本机 Strata 形状， 的可选适配器）。
// 探不到就返回 known=false，调用方降级为只看窗口状态。
func (e *Engine) serviceBusy() (busy bool, known bool) {
	base, _ := e.st.GetSetting("service_status_url")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	hostport := strings.TrimPrefix(strings.TrimPrefix(base, "http://"), "https://")
	hostport = strings.Trim(hostport, "/")
	if i := strings.Index(hostport, "/"); i >= 0 {
		hostport = hostport[:i]
	}
	return tierBusy(hostport)
}

// providerBusy 查「某个窗口用的那个服务商」忙不忙。云端/探不到 → known=false，调用方不拦。
// 档位切换器（本机 8080）自己不干推理，先问它当前档在哪个端口，再查那个端口。
func (e *Engine) providerBusy(provider string) (busy bool, known bool) {
	if provider == "" {
		return false, false
	}
	local, hostport := e.cfg.ProviderIsLocal(provider)
	if !local || hostport == "" {
		return false, false
	}
	var st struct {
		Current string         `json:"current"`
		Tiers   map[string]int `json:"tiers"`
	}
	if getJSON("http://"+hostport+"/__status", &st) && st.Current != "" {
		if p, ok := st.Tiers[st.Current]; ok {
			hostport = fmt.Sprintf("127.0.0.1:%d", p)
		}
	}
	return busyAt(hostport)
}

// tierBusy 查档位切换器背后那一档的忙闲。
func tierBusy(switcher string) (bool, bool) {
	var st struct {
		Current string         `json:"current"`
		Tiers   map[string]int `json:"tiers"`
	}
	if !getJSON("http://"+switcher+"/__status", &st) || st.Current == "" {
		return false, false
	}
	port, ok := st.Tiers[st.Current]
	if !ok {
		return false, false
	}
	return busyAt(fmt.Sprintf("127.0.0.1:%d", port))
}

// busyAt 查某个 主机:端口 上的推理服务忙不忙。
func busyAt(hostport string) (bool, bool) {
	var s struct {
		Busy   *bool `json:"busy"`
		Queued *int  `json:"queued"`
	}
	if !getJSON(fmt.Sprintf("http://%s/status", hostport), &s) || s.Busy == nil {
		return false, false
	}
	q := 0
	if s.Queued != nil {
		q = *s.Queued
	}
	return *s.Busy || q > 0, true
}

// providerAlive 只判本地服务商端口通不通；云端判不了。
func (e *Engine) providerAlive(provider string) (alive bool, known bool) {
	if provider == "" {
		return false, false
	}
	local, hostport := e.cfg.ProviderIsLocal(provider)
	if !local || hostport == "" {
		return false, false
	}
	c, err := net.DialTimeout("tcp", hostport, 1500*time.Millisecond)
	if err != nil {
		return false, true
	}
	c.Close()
	return true, true
}

func getJSON(url string, out any) bool {
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false
	}
	return json.Unmarshal(b, out) == nil
}

func displayName(w openclaw.Window) string {
	if w.Label != "" {
		return w.Label
	}
	return shortKey(w.Key)
}

// attachKind 按扩展名给出 MIME 与文件名（网关按 mimeType 分图片/文档/其他）。
func attachKind(path string) (mime, name string) {
	name = filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		mime = "image/png"
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".gif":
		mime = "image/gif"
	case ".webp":
		mime = "image/webp"
	case ".bmp":
		mime = "image/bmp"
	case ".pdf":
		mime = "application/pdf"
	case ".txt", ".md", ".markdown":
		mime = "text/plain"
	case ".csv", ".tsv":
		mime = "text/csv"
	case ".json":
		mime = "application/json"
	case ".log":
		mime = "text/plain"
	case ".doc":
		mime = "application/msword"
	case ".docx":
		mime = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		mime = "application/vnd.ms-excel"
	case ".xlsx":
		mime = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".ppt":
		mime = "application/vnd.ms-powerpoint"
	case ".pptx":
		mime = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".zip":
		mime = "application/zip"
	default:
		mime = "application/octet-stream"
	}
	return mime, name
}

func shortKey(k string) string {
	if i := strings.LastIndex(k, ":"); i >= 0 && len(k)-i <= 9 {
		return "…" + k[i+1:]
	}
	if len(k) > 28 {
		return k[:12] + "…" + k[len(k)-12:]
	}
	return k
}
