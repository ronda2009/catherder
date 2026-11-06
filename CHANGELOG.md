# 更新记录

版本号读项目根下的 `VERSION`，编译时注入界面显示。

## 1.0.7 — 2026-11-06

- **服务忙闲改成通用识别**：原来只认 `/status`（Strata 那类路由自己写的接口），别人机器上常见的 llama.cpp、ollama、llama-swap 都没有这个口，一律「探不到」，服务闸门形同不存在。现在按服务自己会的问法去问：`/status` → `/slots`（llama.cpp）→ `/api/ps`（ollama）→ `/running`（llama-swap，再追一层问它代管的那个服务）；四种都没答才降级只看窗口。顶栏「服务」悬停写明认成了哪种、凭什么这么说。
- **设置页新增「服务状态地址」**：以前只能走命令行 `set`，换了端口就只能猜。
- **修：「上移 / 下移」跟屏幕方向相反**。队列是新的在上、最下面那条先派，按钮原先按派发顺序算，于是点「上移」卡片反而往下跳。现在按屏幕方向：**上移＝卡片往上走一格（＝后派一位），下移＝卡片往下走一格（＝提前派）**。按钮名字与位置没变。
- **异常任务新增「取消」**：横幅上「继续派发」旁边加了一个「取消」，异常任务区每条也加了「取消」。点取消＝**这批异常不再派发、也不删除，就留在异常任务里摆着**，横幅跟着一起复位；以后还能随时「重试」或「删除」。命令行版同步加了 `dismiss <id>`。
- **修：异常任务把自动关机拦没了**（自动关机权限大于异常提醒）。关机条件不再看有没有异常任务：队列清空、窗口都闲就照常关机。没取消的异常第二天开机自启后继续弹横幅提示，直到点「取消」或「删除」才不再提示。
- **修：每 60 秒的自动刷新会把正在打字的输入框刷掉**（光标没了，要用鼠标点一下才能接着写）。现在后台刷新时只要焦点在输入框或下拉里，数据照读、列表不重建；用户自己点的操作（加入队列、上移/下移、删除、编辑、保存设置）照旧立刻重画。

## 1.0.6 — 2026-11-02

- 软件启动时自动取消「自动关机」：开机后队列必空、窗口必闲，条件秒满足，不取消就会在开机两三分钟后再关一次机。启动时撤掉挂着的待关机、同时把勾选取消并保存（横幅不再出现）；要用就在设置里重新勾选。

## 1.0.5 — 2026-11-01

- 「取消关机」按钮：撤掉这次待关机，同时取消「自动关机」勾选并保存，横幅随即消失。
- 图标补 128×128 与 256×256 两层（大图标视图不再被拉糊、不再比同排程序小）；图标资源同步用于窗口、任务栏、托盘与界面。

## 1.0.4 — 2026-10-28

- 截图标注新增「箭头」：拖一条线，末端带箭头，箭头大小随「细／中／粗」变。
- 撤销、重做、清空、存盘、另存为、合成出图沿用原通路。

## 1.0.3 — 2026-10-28

- 关机命令改为 `shutdown /s /t 0 /f`：带强制，挡关机的程序不再卡住关机。代价是不给该程序保存机会。
- 显卡采集换成厂商中立方案：Windows 性能计数器为主，`nvidia-smi` 兜底。

## 1.0.2 — 2026-10-28

- 排队卡片按钮改名「上移／下移／编辑」，悬停说明一句话讲清用途。
- 未收到回执的任务照常进「已完成」，不再滞留。

## 1.0.1 — 2026-10-27

- 界面语言 B 阶段：托盘提示、系统对话框、运行消息跟随所选语言。
- 排队调序修复；任务栏图标修复（图标组编号）。
- 正式版与测试版双实例并存，各用各的库，互不干扰。

## 1.0.0 — 2026-10-27

- 首个正式版：免安装单文件 exe，界面底部显示版本号与本版编译时刻。
- 界面语言 A 阶段：界面文字中英切换，保存即生效；OpenClaw 传来的数据保持原文。
- 截图工具：拖框选区、画笔／方形／椭圆／文字／马赛克／取色器、粗细三档、撤销重做、另存为。
- 带图与带文件任务：先进待发区，补文字后入队，附件随任务发出。

## 0.x — 2026-10-23 至 2026-10-26

- 核心引擎：入队 → 自动派发 → 判完工 → 回执 → 转发。只读 OpenClaw 会话库，派发走网关 RPC，不跑模型回合。
- 界面版：派工／窗口／日志／设置四页，卡片式任务单，开机自启。
- 失败分诊：异常统一停机，界面红横幅，可「重试这条」或「继续派发」。
- 同模型并发确认弹窗，含「今天不再提醒」「本次运行不再提醒」。
- 自动关机：条件齐才武装，60 秒缓冲，有新活动自动撤销，可选时段。
- 导出 CSV（UTF-8 BOM），换机器可移植探测（无通道可选「不用通知」，未装模型服务状态适配器照常干活）。

---

# Changelog

Version string comes from the `VERSION` file and is injected at build time.

## 1.0.4 — 2026-10-28

- Screenshot annotation: added arrow. Drag a line, arrow head at the end, head size follows the thin/medium/bold setting.
- Undo, redo, clear, save, save as and compositing unchanged.

## 1.0.3 — 2026-10-28

- Shutdown command changed to `shutdown /s /t 0 /f`: forced, so programs that block shutdown no longer stall it. Trade-off: they get no chance to save.
- GPU utilisation made vendor neutral: Windows performance counters first, `nvidia-smi` as fallback.

## 1.0.2 — 2026-10-28

- Queue card buttons renamed Earlier / Later / Edit (上移／下移／编辑), tooltips reduced to one line.
- Tasks without acknowledgement now move into Done instead of lingering.

## 1.0.1 — 2026-10-27

- Interface language phase B: tray hints, system dialogs and run messages follow the selected language.
- Queue reordering fixed; taskbar icon fixed (icon group index).
- Release and test instances can run side by side with separate databases.

## 1.0.0 — 2026-10-27

- First release: portable single-file exe, version and build time shown at the bottom of the window.
- Interface language phase A: interface text switches between Chinese and English on save; data from OpenClaw stays untranslated.
- Screenshot tool: drag a region, pen / rectangle / ellipse / text / mosaic / colour picker, three widths, undo, redo, save as.
- Image and file tasks: pending area first, add a note, then queue; attachments are sent with the task.

## 0.x — 2026-10-23 to 2026-10-26

- Core engine: queue → auto dispatch → completion detection → acknowledgement → forward. Reads the OpenClaw session database read-only, dispatches over gateway RPC, never runs a model turn.
- GUI: Dispatch / Windows / Log / Settings, card-style task list, autostart.
- Failure triage: any failure halts dispatch with a red banner, Retry this / Resume.
- Same-model conflict confirmation dialog with "not today" and "not this run".
- Automatic shutdown: armed only when conditions are met, 60 s buffer, cancelled automatically by new activity, optional time window.
- CSV export (UTF-8 BOM); portable detection on a new machine (no channel → "no notifications", missing service status adapter degrades gracefully).
