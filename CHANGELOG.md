# 更新记录

版本号读项目根下的 `VERSION`，编译时注入界面显示。

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
