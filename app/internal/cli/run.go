// 派工台（CatHerder）命令行版 —— M1 核心引擎的入口与验收工具。
// 界面版复用同一套 internal/*：openclaw(读库+网关) / store(任务记录) / engine(派发调度)。
// 用法见 usage()。全程不跑模型回合：读库是只读，发消息走网关 RPC。
package cli

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"catherder/internal/engine"
	"catherder/internal/openclaw"
	"catherder/internal/store"
)

// Commands 给入口程序判断「这个词是命令行子命令」。
var Commands = []string{"scan", "include", "exclude", "add", "queue", "move", "cancel",
	"errors", "retry", "confirm", "export", "tick", "run", "set", "get", "log", "notify", "status", "shot"}

// IsCommand 判断 argv[0] 是不是子命令（不是就走界面）。
func IsCommand(word string) bool {
	for _, c := range Commands {
		if c == word {
			return true
		}
	}
	return false
}

type opts struct {
	config string
	db     string
	data   string
}

// Run 跑命令行模式；argv 是去掉程序名之后的参数。
func Run(argv []string) {
	o := &opts{}
	fs := flag.NewFlagSet("catherder", flag.ExitOnError)
	fs.StringVar(&o.config, "config", openclaw.DefaultConfigPath(), "openclaw.json 路径")
	fs.StringVar(&o.db, "db", openclaw.DefaultDBPath("main"), "OpenClaw 会话库路径（只读）")
	fs.StringVar(&o.data, "data", store.DefaultPath(), "派工台自己的数据库路径")
	fs.Usage = usage
	if err := fs.Parse(argv); err != nil {
		os.Exit(2)
	}
	args := fs.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	cfg, err := openclaw.LoadConfig(o.config)
	if err != nil {
		die("读取 OpenClaw 配置失败：%v", err)
	}
	st, err := store.Open(o.data)
	if err != nil {
		die("打开派工台数据库失败：%v", err)
	}
	defer st.Close()
	eng := engine.New(cfg, st, o.db)

	switch args[0] {
	case "scan":
		cmdScan(st, eng, args[1:])
	case "include":
		cmdInclude(st, eng, args[1:], true)
	case "exclude":
		cmdInclude(st, eng, args[1:], false)
	case "add":
		cmdAdd(st, eng, args[1:])
	case "queue":
		cmdQueue(st)
	case "move":
		cmdMove(st, args[1:])
	case "cancel":
		cmdCancel(st, args[1:])
	case "errors":
		cmdErrors(st)
	case "retry":
		cmdRetry(st, args[1:])
	case "confirm":
		cmdConfirm(st, eng, args[1:])
	case "tick":
		cmdTick(eng, args[1:])
	case "run":
		cmdRun(st, eng, args[1:])
	case "set":
		cmdSet(st, args[1:])
	case "get":
		cmdGet(st, args[1:])
	case "log":
		cmdLog(st, args[1:])
	case "notify":
		cmdNotify(cfg, st, args[1:])
	case "export":
		cmdExport(st, args[1:])
	case "status":
		cmdStatus(cfg, st, eng)
	case "shot":
		cmdShot(st, args[1:])
	default:
		usage()
		os.Exit(2)
	}
}

// Usage 给入口程序用：参数不对时打印人话用法，别让程序掉进界面模式去甩 Go 栈。
func Usage() { usage() }

func usage() {
	fmt.Println(`工头派工台 CatHerder —— 自动跟踪 OpenClaw 窗口，窗口空闲时派发下一条任务

  catherder scan [--pick]            列出可派工窗口（--pick 附序号便于勾选）
  catherder include <序号|窗口键>…    纳入派工范围
  catherder exclude <序号|窗口键>…    移出派工范围
  catherder add <序号> <内容>         给指定窗口添加一条任务（内容用 - 从标准输入读取）
  catherder queue                    查看队列（未派发，按提交顺序）
  catherder move <id> <到谁前面|0>    调序（0=队首）
  catherder cancel <id>              撤销未派发或标记为异常的任务
  catherder errors                   查看标记为异常的任务（含原因）
  catherder retry <id>               将标记为异常的任务重新排队
  catherder confirm [--yes|--no]     查看待确认的并发项；--yes 仍然派发，--no 暂不派发
                     [--mute today|run]  同时设为不再提醒
  catherder tick [--dry]             执行一次检查（--dry 只查看不改动）
  catherder run [--interval 60]      常驻运行：每隔 N 秒自动检查一次
  catherder set <键> <值>            设置（见 status 里的键清单）
  catherder get <键>                 读设置
  catherder log [条数]               查看日志
  catherder export [路径]            导出任务记录 CSV（省略路径时存到 文档\派工台记录-日期时间.csv）
  catherder notify <内容>            测试通知通道（实际发送一条）
  catherder status                   查看当前配置与状态`)
}

func die(f string, a ...any) {
	fmt.Fprintf(os.Stderr, f+"\n", a...)
	os.Exit(1)
}

// ---- scan / 勾选 ----

type listed struct {
	key   string
	label string
	model string
	prov  string
	state string
	inc   bool
}

// listSchedulable 拿“可派工窗口 + 勾选状态”，按最近活动排序，序号从 1 起。
// 顺手把扫到的窗口写进派工台自己的库：勾选与 add 靠序号时才认得它们。
func listSchedulable(st *store.Store, eng *engine.Engine) []listed {
	db, err := openclaw.OpenDB(eng.DBPath())
	if err != nil {
		die("读取会话库失败：%v", err)
	}
	defer db.Close()
	wins, err := db.Windows()
	if err != nil {
		die("扫描窗口失败：%v", err)
	}
	inc := map[string]bool{}
	for _, p := range must(st.Included()) {
		inc[p.Key] = true
	}
	var out []listed
	for _, w := range wins {
		if !w.Schedulable() || w.Archived {
			continue // 归档的窗口不列也不派（和界面、OpenClaw 控制界面一致）
		}
		st.UpsertWindow(w.Key, w.Label, w.Status, w.Model, w.Provider, true)
		out = append(out, listed{key: w.Key, label: w.Label, model: w.Model, prov: w.Provider,
			state: stateText(w), inc: inc[w.Key]})
	}
	return out
}

func stateText(w openclaw.Window) string {
	if !w.Idle() {
		return "执行中"
	}
	if w.Pending > 0 {
		return fmt.Sprintf("空闲（待处理 %d 条）", w.Pending)
	}
	return "空闲"
}

func cmdScan(st *store.Store, eng *engine.Engine, args []string) {
	pick := len(args) > 0 && args[0] == "--pick"
	list := listSchedulable(st, eng)
	for i, l := range list {
		mark := " "
		if l.inc {
			mark = "☑"
		}
		name := l.label
		if name == "" {
			name = "…" + tail(l.key)
		}
		fmt.Printf("%2d %s %-22s %-10s %-14s %s\n", i+1, mark, clip(name, 22), stateTextOf(l), clip(l.model, 14), l.prov)
		if !pick {
			fmt.Printf("      %s\n", l.key)
		}
	}
	fmt.Printf("共 %d 个可派工窗口（心跳、定时任务、子代理窗口已排除）\n", len(list))
}

func stateTextOf(l listed) string { return l.state }

func cmdInclude(st *store.Store, eng *engine.Engine, args []string, on bool) {
	if len(args) == 0 {
		die("要给窗口序号或窗口键")
	}
	for _, a := range args {
		key := resolveWindow(st, eng, a)
		if err := st.SetIncluded(key, on); err != nil {
			die("更新勾选状态失败：%v", err)
		}
		if on {
			fmt.Println("已纳入派工范围：", key)
		} else {
			fmt.Println("已移出派工范围：", key)
		}
	}
}

// resolveWindow 允许写序号（scan --pick 里的数字）或完整窗口键。
// 序号走的是 scan 同一份清单，所以两边编号永远一致。
func resolveWindow(st *store.Store, eng *engine.Engine, arg string) string {
	if n, err := strconv.Atoi(arg); err == nil {
		list := listSchedulable(st, eng)
		if n >= 1 && n <= len(list) {
			return list[n-1].key
		}
		die("窗口序号 %d 不在范围内（1~%d），请先运行 scan --pick 查看编号", n, len(list))
	}
	return arg
}

// ---- 任务 ----

func cmdAdd(st *store.Store, eng *engine.Engine, args []string) {
	if len(args) < 2 {
		die("用法：add <窗口序号或键> <内容>（内容用 - 从标准输入读取）")
	}
	key := resolveWindow(st, eng, args[0])
	body := strings.Join(args[1:], " ")
	if body == "-" {
		body = readStdin()
	}
	body = strings.TrimSpace(body)
	if body == "" {
		die("任务内容不能为空")
	}
	label := labelOf(st, key)
	id, err := st.AddTask(body, key, label)
	if err != nil {
		die("加入队列失败：%v", err)
	}
	fmt.Printf("T-%03d 已加入队列 → %s\n", id, firstNonEmpty(label, key))
	_ = eng
}

func labelOf(st *store.Store, key string) string {
	for _, p := range must(st.Included()) {
		if p.Key == key {
			return p.Label
		}
	}
	for _, r := range must(st.WindowsAll()) {
		if r.Key == key {
			return r.Label
		}
	}
	return ""
}

func cmdQueue(st *store.Store) {
	q := must(st.Queue())
	d := must(st.Dispatched())
	if len(q) == 0 && len(d) == 0 {
		fmt.Println("队列为空")
		return
	}
	for _, t := range d {
		fmt.Printf("T-%03d [已派发] → %s\n", t.ID, firstNonEmpty(t.TargetLabel, t.TargetKey))
	}
	for _, t := range q {
		fmt.Printf("T-%03d [%s] → %s\n", t.ID, t.State, firstNonEmpty(t.TargetLabel, t.TargetKey))
		fmt.Printf("      %s\n", clip(oneLine(t.Body), 100))
	}
}

func cmdMove(st *store.Store, args []string) {
	if len(args) < 2 {
		die("用法：move <任务id> <移到谁前面|0>")
	}
	id := atoi(args[0])
	before := atoi(args[1])
	if err := st.Move(id, before); err != nil {
		die("调整顺序失败：%v", err)
	}
	fmt.Printf("T-%03d 已移至 T-%03d 之前\n", id, before)
}

func cmdCancel(st *store.Store, args []string) {
	if len(args) < 1 {
		die("用法: cancel <任务id>")
	}
	if err := st.Cancel(atoi(args[0])); err != nil {
		die("撤销失败：%v", err)
	}
	fmt.Printf("T-%03d 已撤销\n", atoi(args[0]))
}

// cmdExport 导任务记录 CSV（不弹框，直接写路径或存到「文档」）。
func cmdExport(st *store.Store, args []string) {
	path := ""
	if len(args) > 0 {
		path = args[0]
	}
	if path == "" {
		dir, err := os.UserHomeDir()
		if err != nil {
			dir = "."
		}
		path = filepath.Join(dir, "Documents", fmt.Sprintf("派工台记录-%s.csv", time.Now().Format("200601-1504")))
	}
	n, err := st.ExportCSV(path)
	if err != nil {
		die("导出失败：%v", err)
	}
	fmt.Printf("已导出 %d 条 → %s\n", n, path)
}

// cmdErrors 列标异常的任务与原因（失败分诊的落地入口）。
func cmdErrors(st *store.Store) {
	es, err := st.RecentErrors(50)
	if err != nil {
		die("读取异常任务失败：%v", err)
	}
	if len(es) == 0 {
		fmt.Println("没有标记为异常的任务。")
		return
	}
	for _, t := range es {
		fmt.Printf("T-%03d → %s\n    原因：%s\n    内容：%s\n", t.ID, t.TargetLabel, clip(oneLine(t.Note), 200), clip(oneLine(t.Body), 100))
	}
	fmt.Println("修复后可用 retry <id> 重新排队，或用 cancel <id> 删除。")
}

func cmdRetry(st *store.Store, args []string) {
	if len(args) < 1 {
		die("用法: retry <任务id>")
	}
	id := atoi(args[0])
	if err := st.RetryTask(id); err != nil {
		die("重新排队失败：%v", err)
	}
	fmt.Printf("T-%03d 已重新排队\n", id)
}

// cmdConfirm 看待确认的同模型并发问题并答复（界面没开着时用）。
func cmdConfirm(st *store.Store, eng *engine.Engine, args []string) {
	r := eng.PendingConfirm()
	if r == nil {
		fmt.Println("没有待确认的并发项。")
		return
	}
	if len(args) == 0 {
		fmt.Printf("T-%03d → %s\n%s\n使用 confirm --yes 仍然派发，--no 暂不派发（可加 --mute today|run）\n", r.TaskID, r.Target, r.Question)
		return
	}
	accept := args[0] == "--yes"
	if !accept && args[0] != "--no" {
		die("用法: confirm [--yes|--no] [--mute today|run]")
	}
	mute := ""
	if i := indexOf(args, "--mute"); i >= 0 && i+1 < len(args) {
		mute = args[i+1]
	}
	if err := eng.AnswerConfirm(accept, mute); err != nil {
		die("提交答复失败：%v", err)
	}
	fmt.Printf("T-%03d %s\n", r.TaskID, map[bool]string{true: "已确认并发，将继续派发", false: "暂不派发，保留队列"}[accept])
}

func indexOf(xs []string, want string) int {
	for i, x := range xs {
		if x == want {
			return i
		}
	}
	return -1
}

// ---- 引擎 ----

func cmdTick(eng *engine.Engine, args []string) {
	if err := eng.Tick(len(args) > 0 && args[0] == "--dry"); err != nil {
		die("%v", err)
	}
}

func cmdRun(st *store.Store, eng *engine.Engine, args []string) {
	interval := 60
	for i := 0; i < len(args); i++ {
		if args[i] == "--interval" && i+1 < len(args) {
			interval = atoi(args[i+1])
			i++
		}
	}
	if interval < 15 {
		interval = 15
	}
	fmt.Printf("工头派工台已开始跟踪窗口，每 %d 秒检查一次（Ctrl+C 停止）\n", interval)
	for {
		if err := eng.Tick(false); err != nil {
			st.Log("tick", "出错: %v", err)
			fmt.Println("本次检查出错：", err)
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
}

// ---- 设置 / 日志 / 通知 ----

func cmdSet(st *store.Store, args []string) {
	if len(args) < 2 {
		die("用法: set <键> <值>")
	}
	if err := st.SetSetting(args[0], strings.Join(args[1:], " ")); err != nil {
		die("保存设置失败：%v", err)
	}
	fmt.Printf("%s = %s\n", args[0], strings.Join(args[1:], " "))
}

func cmdGet(st *store.Store, args []string) {
	if len(args) < 1 {
		die("用法: get <键>")
	}
	v, _ := st.GetSetting(args[0])
	fmt.Printf("%s = %q\n", args[0], v)
}

func cmdLog(st *store.Store, args []string) {
	n := 20
	if len(args) > 0 {
		n = atoi(args[0])
	}
	for _, l := range must(st.RecentEvents(n)) {
		fmt.Println(l)
	}
}

func cmdNotify(cfg *openclaw.Config, st *store.Store, args []string) {
	if len(args) < 1 {
		die("用法：notify <内容>")
	}
	ch, _ := st.GetSetting("notify_channel")
	if ch == "" {
		die("尚未设置 notify_channel（请先运行 status 查看探测到的通道，再执行 set notify_channel <名称>）")
	}
	ws, err := openclaw.Dial(cfg)
	if err != nil {
		die("连接网关失败：%v", err)
	}
	defer ws.Close()
	tg, _ := st.GetSetting("notify_target")
	ref, err := ws.FindConversation("main", ch, tg)
	if err != nil {
		die("查找通道地址失败：%v", err)
	}
	if ref == "" {
		die("在 %s 中未找到目标地址（请确认 target 是否正确，可运行 conversations 查看可用地址）", ch)
	}
	if err := ws.SendToConversation("main", ref, strings.Join(args, " ")); err != nil {
		die("发送失败：%v", err)
	}
	fmt.Println("已发送至", ch, ref)
}

func cmdStatus(cfg *openclaw.Config, st *store.Store, eng *engine.Engine) {
	ch := cfg.EnabledChannels()
	fmt.Println("网关：端口", cfg.Gateway.Port, "鉴权方式", cfg.Gateway.Auth.Mode)
	b, _ := json.Marshal(ch)
	fmt.Println("配置中的通道：", string(b))
	fmt.Println("已纳入派工范围的窗口：")
	for _, p := range must(st.Included()) {
		fmt.Printf("  %s  %s\n", p.Key, p.Label)
	}
	fmt.Println("设置项（未设置的采用默认值）：")
	for _, kv := range [][2]string{
		{"notify_channel", "（空＝不发通知）"},
		{"notify_target", "（空＝该通道第一个地址）"},
		{"max_parallel", "1 —— 同时放行几个窗口"},
		{"recheck_seconds", "60 —— 空闲后复核多少秒"},
		{"retry_delay_seconds", "10 —— 派发失败后隔多久重试"},
		{"append_tail", "1 —— 派发时在任务内容末尾追加完成回执要求"},
		{"gate_service", "1 —— 模型服务忙时不派发新任务"},
		{"gate_gpu", "1 —— 显卡忙时不派发新任务"},
		{"gpu_busy_percent", "30 —— 显卡利用率连续两轮超过这个数才算忙"},
		{"auto_include_new", "0 —— 新扫到的窗口自动纳入"},
		{"service_status_url", "http://127.0.0.1:8080 —— 模型服务状态地址（探不到自动降级）"},
		{"auto_shutdown", "0 —— 全部任务完成且所有窗口空闲后自动关机"},
		{"shutdown_hours", "（空＝不限时段）例 22:00-06:00"},
		{"mute_mode", "（空＝每次都提醒）today/run —— 同模型并发提醒不再弹"},
		{"paused", "（空＝正常派发）"},
		{"halt", "（空＝正常派发）—— 全部异常时停止派发等待处理"},
	} {
		v, _ := st.GetSetting(kv[0])
		if v == "" {
			v = "〔默认〕" + kv[1]
		}
		fmt.Printf("  %-20s %s\n", kv[0], v)
	}
	if busy, known := eng.ServiceBusyForStatus(); known {
		fmt.Println("模型服务: 正忙 =", busy)
	} else {
		fmt.Println("模型服务: 这台机器探不到状态接口，只看窗口状态")
	}
	// 显卡闸（gate_gpu）：探不到（老驱动 / 没显卡计数器 / 不是相关显卡）就说探不到，不拦派发。
	if busy, known, why := eng.GpuBusyForStatus(); known {
		if busy {
			fmt.Println("显卡: 正忙 ——", why)
		} else {
			fmt.Println("显卡: 空闲（利用率未连续超阈值）")
		}
	} else {
		fmt.Println("显卡: 这台机器读不到利用率（系统显卡计数器不可用，nvidia-smi 也没有），不拦派发")
	}
	if c, err := st.Counts(); err == nil {
		fmt.Printf("任务计数：排队=%d 执行中=%d 异常=%d 已完成=%d 已删除=%d\n",
			c["queued"], c["dispatched"], c["error"], c["done"]+c["done_unacked"], c["canceled"])
	}
	if s := eng.ShutdownState(); s != "" {
		fmt.Println("自动关机:", s)
	}
	if r := eng.PendingConfirm(); r != nil {
		fmt.Printf("等同模型确认: T-%03d → %s（%s）\n", r.TaskID, r.Target, r.Endpoint)
	}
}

// ---- 小工具 ----

func readStdin() string {
	sc := bufio.NewScanner(os.Stdin)
	var sb strings.Builder
	for sc.Scan() {
		sb.WriteString(sc.Text())
		sb.WriteString("\n")
	}
	return sb.String()
}

func must[T any](v T, err error) T {
	if err != nil {
		die("%v", err)
	}
	return v
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func tail(k string) string {
	if i := strings.LastIndex(k, ":"); i >= 0 {
		return k[i+1:]
	}
	return k
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
