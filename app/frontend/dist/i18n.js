/* 界面语言（A 阶段：只翻界面文字）。
   做法：中文原文当键，英文按「整段优先」替换；数字与变量不受影响。
   只在前端显示层动手：Go 传来的任务内容、日志、窗口名不翻（那是 B/C 阶段）。 */
'use strict';

let uiLang = 'zh';

/* 词典：中文 → 英文。长句排前面（脚本会自动按长度排，这里只管写）。
   带 {n} 的是可变数字位置，替换时保留。 */
const DICT = {
  /* ---- 顶栏与标签页 ---- */
  // 整句先登记（词典按长度从长到短替换，短句「工头派工台」会把含它的长句切碎）。
  '工头派工台 CatHerder': 'Foreman CatHerder',
  '工头派工台': 'Foreman',
  'CatHerder · 为 OpenClaw 窗口派工': 'CatHerder · dispatch to OpenClaw windows',
  '工头派工台图标': 'Foreman icon',
  '小工厂': 'Little factory',
  '派工': 'Dispatch',
  '窗口': 'Windows',
  '日志': 'Log',
  '设置': 'Settings',

  /* ---- 顶栏徽章 ---- */
  '暂停自动派工': 'Pause dispatch',
  '开始自动派工': 'Resume dispatch',
  '派工中 每 ': 'Running · every ',
  '秒': 's',
  '已暂停': 'Paused',
  '服务空闲': 'Service idle',
  '服务忙': 'Service busy',
  '服务未知': 'Service unknown',
  '同时 ': 'Parallel ',
  ' 窗': ' windows',
  '个窗口': 'windows',
  '同时最多给几个窗口派工（在设置页改）': 'Max windows dispatched at once (change in Settings)',
  '排队 ': 'Queued ',
  '（异常 ': ' (failed ',
  '执行 ': 'Running ',
  '完成 ': 'Done ',
  '下次 ': 'Next in ',
  '已等 ': 'Waiting ',
  ' 秒前': 's ago',
  ' 分钟前': 'm ago',
  ' 小时前': 'h ago',
  ' 天前': 'd ago',
  ' 分 ': 'm ',
  ' 小时 ': 'h ',
  ' 秒': 's',
  ' 分': 'm',
  ' 小时': 'h',
  '本版编译于 ': 'Built ',
  '版本 ': 'Version ',

  /* ---- 横幅 ---- */
  '⛔ 已停止派发：': '⛔ Dispatch halted: ',
  '。修复后点「重试」，或点「继续派发」。': '. Fix it, then press "Retry" or "Resume".',
  '继续派发': 'Resume',
  '🌙 自动关机：': '🌙 Auto shutdown: ',
  '取消关机': 'Cancel shutdown',

  /* ---- 异常区 ---- */
  '没有标异常的任务。': 'No failed tasks.',
  '派发失败': 'Dispatch failed',
  ' · 重试 ': ' · retried ',
  ' 次': ' times',
  '原因：': 'Reason: ',
  '未记录': 'not recorded',
  '重试': 'Retry',
  '删除': 'Delete',

  /* ---- 派工页 ---- */
  '新建任务': 'New task',
  '先选择窗口，再填写内容，按 Ctrl+Enter 加入队列；可继续选其他窗口接着写。队列按加入顺序执行。':
    'Pick a window, write the task, press Ctrl+Enter to queue it. You can switch windows and keep writing. The queue runs in order.',
  '请写清楚任务内容，越具体越好。按 Ctrl+Enter 加入队列。':
    'Describe the task clearly — the more specific, the better. Ctrl+Enter to queue.',
  '加入队列': 'Queue it',
  '按 Ctrl+Enter 加入队列': 'Press Ctrl+Enter to queue',
  '传文件': 'Attach files',
  '选文件随任务一起发出（可多选；也可以直接把文件拖进来）':
    'Pick files to send with the task (multiple allowed; you can also drop them here)',
  '带图/带文件任务：附件跟文字一起发给目标窗口（窗口那边能直接看到）。文件可直接拖进这一块。':
    'Tasks with images/files: attachments go to the target window together with the text (it can see them there). Drop files right into this box.',
  '截图': 'Screenshot',
  '像微信那样截屏：拖框选区、画圈写字，完成后作为带图任务发出':
    'Capture like WeChat: drag a region, draw and type, then send it as a task with an image',
  '任务内容尚未填写。': 'Task text is empty.',
  '请先去「窗口」页勾选要派工的窗口。': 'Tick the windows to dispatch on the "Windows" page first.',
  '（尚未勾选任何窗口，请先去「窗口」页勾选）': '(no window ticked yet — tick them on the "Windows" page first)',
  '空闲': 'idle',
  '执行中': 'busy',
  '模型未知': 'model unknown',
  '待处理输入 ': 'pending input ',
  ' 条': ' items',
  '纳入': 'included',
  '移出': 'excluded',
  '请看这个文件（已随本条一起发出）': 'Please look at this file (sent along with this message)',

  /* ---- 待发附件 ---- */
  '准备随任务发出 ': 'Ready to send ',
  ' 个文件（点缩略图放大，点右上角 × 拿掉）': ' file(s) with this task (click a thumbnail to enlarge, click × at its corner to remove)',
  '点一下放大': 'Click to enlarge',
  '（点一下放大）': ' (click to enlarge)',
  '不要这个附件': 'Remove this attachment',
  '读不到图': 'cannot read image',
  '文件': 'file',
  '图': 'IMG',
  '拖进来 ': 'Dropped ',
  ' 个文件，已列入待发附件。': ' file(s), added as attachments.',
  '拖进来的文件：': 'Dropped files:',

  /* ---- 队列区 ---- */
  '排队中': 'Queued',
  '排队第 ': 'Queued #',
  '共 ': 'Total ',
  ' 条，按顺序执行': ' item(s), run in order',
  /* 各区改成新的在上 */
  ' 条 · 新的在上 · 最下面那条先派': ' item(s) · newest on top · the bottom one dispatches first',
  '队列为空': 'Queue is empty',
  '队列为空。在上方填写任务内容，按 Ctrl+Enter 加入队列。':
    'Queue is empty. Write a task above and press Ctrl+Enter to queue it.',
  '上移': 'Up',
  '下移': 'Down',
  '编辑': 'Edit',
  '已派发，等待完成': 'Dispatched, waiting',
  '已完成 ✅': 'Done ✅',
  '已完成（未收到回执）': 'Done (no receipt)',
  '已删除': 'Deleted',
  '派发 ': 'Dispatched ',
  '完成 ': 'Done ',

  /* ---- 已派发 / 已完成 ---- */
  '已派发待回复': 'Dispatched, awaiting reply',
  '本轮开程序以来派出去、还没回到已完成的任务。点任务内容可展开看全文，隔一会儿自动收回。':
    'Tasks dispatched since this program started that have not come back as done. Click the text to expand; it collapses again shortly.',
  '点一下看全文，几秒后自动收回': 'Click to see the full text; it collapses after a few seconds',
  '没有待回复的任务。': 'No tasks awaiting reply.',
  '已完成': 'Done',
  '本轮还没有完成的任务。': 'Nothing finished in this round yet.',
  ' 条已完成（没收到回执的也在这里，标签会写明）。点内容可展开看全文。':
    ' task(s) done (ones with no receipt are here too — the label says so). Click the text to expand.',

  /* ---- 窗口页 ---- */
  '可派工的窗口': 'Dispatchable windows',
  '勾选后才会派工。心跳、定时任务、子代理窗口不参与派工（避免自我派发）。新扫描到的窗口默认不勾选。':
    'Only ticked windows get work. Heartbeat, scheduled jobs and sub-agent windows are excluded (to avoid self-dispatch). Newly found windows start unticked.',
  '未扫描到窗口，请确认 OpenClaw 网关正在运行，再点「立即检查一次」。':
    'No windows found. Make sure the OpenClaw gateway is running, then press "Check now".',
  '立即检查一次': 'Check now',
  '新扫描到的窗口自动纳入派工范围': 'Automatically include newly found windows',

  /* ---- 并发确认弹窗 ---- */
  '同一模型已有任务在执行': 'The same model already has a task running',
  '两个窗口用的是同一个模型、又要放行第二条时才会问；不同模型或云端模型不问。':
    'Asked only when two windows share one model and a second task is about to start; different models or cloud models are not asked.',
  '仍然派发': 'Dispatch anyway',
  '暂不派发，保留队列': 'Keep in queue, do not dispatch',
  '关闭（Esc）': 'Close (Esc)',
  '默认每次都提醒。若不想再被提醒，去「设置 → 同模型并发提醒」改成「今天不再提醒」或「本次运行不再提醒」。':
    'Asked every time by default. To stop being asked, go to Settings → Same-model reminder and pick "Not again today" or "Not again this run".',

  /* ---- 设置页 ---- */
  '修改后点「保存」。': 'Press "Save" after editing.',
  '窗口巡检间隔': 'Check interval',
  '同时最多派给': 'Max parallel windows',
  '派发 2 个窗口时，请先确认两个窗口各自的模型服务均已启动（本程序不切换模型，也不检测模型状态）。':
    'When dispatching to 2 windows, first make sure each window\'s model service is running (this program does not switch models and does not check model state).',
  '完成复核等待': 'Completion recheck wait',
  '派发失败重试间隔': 'Retry delay after failure',
  '派发时在任务末尾追加完成回执要求（便于回收结果）':
    'Append a completion-receipt request at the end of each task (makes results easier to collect)',
  '模型服务繁忙时暂缓派发新任务': 'Hold new tasks while the model service is busy',
  '显卡繁忙时暂缓派发新任务': 'Hold new tasks while the GPU is busy',
  '显卡利用率连续两轮超过阈值就先不派（单轮偶发尖峰不算）。服务没报忙、但显卡还在动的时候能挡住误判。':
    'Pause new dispatches while GPU utilization stays above the threshold for two rounds in a row (a one-round spike does not count). Catches the case where the model service reports idle while the GPU is still working.',
  '显卡利用率阈值': 'GPU utilization threshold',
  '%（连续两轮超过这个数才算忙）': '% (counts as busy only after two consecutive rounds above this)',
  '完成结果通知通道': 'Channel for result notifications',
  '留空＝不发送通知，仅记录在日志中。': 'Empty = no notification, results go to the log only.',
  '不发送通知（仅记录日志）': 'No notification (log only)',
  '（本机未配置）': ' (not configured here)',
  '刷新通道': 'Refresh channels',
  '正在重读本机的通道配置…': 'Re-reading local channel config…',
  '通道已重读；选通道会自动带出该通道的收件地址。': 'Channels re-read; picking one fills its address below.',
  '重读通道失败：': 'Re-reading channels failed: ',
  '收件地址': 'Recipient',
  '收件地址（手填）': 'Recipient (manual)',
  '从上面选会自动填到这里；名单里没有时也可自己填': 'Picking from the list fills this in; you can also type it yourself',
  '刷新地址': 'Refresh list',
  '（选了通道才能查名单）': '(pick a channel first)',
  '选了通知通道后，这里会列出本机 OpenClaw 已知的会话供你选。':
    'After you pick a channel, the conversations known to your OpenClaw are listed here.',
  '正在查本机已知会话…': 'Looking up local conversations…',
  '查名单失败：': 'Looking up the list failed: ',
  '。不影响手填地址。': '. Typing the address by hand still works.',
  '已列出 ': 'Listed ',
  ' 个「': ' conversation(s) of "',
  '」会话；选一个自动填到下面，也可以继续手填。': '"; pick one to fill the box below, or keep typing.',
  '没查到「': 'No conversation found for "',
  '」的会话。可能这个通道还没收过消息；可在下面手填地址，或先在那个通道里发一条消息再点「刷新名单」。':
    '". This channel may not have received any message yet; type the address below, or send one there first and press "Refresh list".',
  '试发一条通知': 'Send a test notification',
  '【派工台】这是一条测试通知，收到即表示通道正常。': '[Foreman] This is a test notification; receiving it means the channel works.',
  '本机不发送通知，结果仅记录在日志中。': 'No notification will be sent from this machine; results are logged only.',
  '同模型并发提醒': 'Same-model reminder',
  '每次都提醒': 'Ask every time',
  '今天不再提醒': 'Not again today',
  '本次运行不再提醒': 'Not again this run',
  '开机自动启动': 'Start with Windows',
  '开机后自动启动本程序（仅写入当前用户的启动项，可随时取消）':
    'Start this program at boot (writes only the current user\'s Run entry; you can undo anytime)',
  '开机自动启动已开启': 'Start with Windows is on',
  '开机自动启动已取消': 'Start with Windows is off',
  '已开启，开机时自动启动的就是当前这份：': 'On — the entry points to this exact copy: ',
  '已开启，但登记的是旧位置：': 'On, but the entry points to an old location: ',
  '（当前运行的是 ': ' (this copy runs from ',
  '）——请取消勾选再勾一次，把登记更新过来。': ') — untick and tick again to update the entry.',
  '读开机启动状态失败：': 'Reading the boot entry failed: ',
  '设置开机自动启动失败：': 'Changing the boot entry failed: ',
  '点窗口右上角叉时收到任务栏图标里（不退出，双击图标可叫回）':
    'Closing the window hides it to the tray (it keeps running; double-click the icon to bring it back)',
  '全部任务完成且所有窗口空闲后自动关机（60 秒缓冲期内如有新活动自动取消）':
    'Shut down when every task is done and all windows are idle (a 60-second buffer cancels itself if anything happens)',
  '关机时段': 'Shutdown window',
  '例：22:00-06:00，留空＝不限时段': 'Example: 22:00-06:00, empty = any time',
  '界面语言': 'Language',
  '界面显示语言（保存后立刻生效，只影响本程序的界面文字）':
    'Interface language (takes effect right after Save; only this program\'s own wording)',
  '提交 ': 'Submitted ',
  '已重新排队。': 're-queued.',
  '已删除。': 'deleted.',
  '已前移。': 'moved up.',
  '已后移。': 'moved down.',
  '已加入队列 ': 'Queued ',
  '，目标窗口空闲时会自动派发。': '; it dispatches when the target window is idle.',
  '已取回输入框，修改后请重新加入队列。': 'Back in the editor — edit it and queue it again.',
  /* 队列里就地编辑 */
  '已转入就地编辑，这条先不派发；改完按「改好了」重新排队。': 'opened for in-place editing — this one is held back; press "Done editing, re-queue" to put it back in line.',
  '改完前这条不会被派发，后面的任务照常走': 'Held until you confirm — the tasks behind it keep going',
  '存好并回到排队，按 Ctrl+Enter 也一样': 'Save it and put it back in line (Ctrl+Enter does the same)',
  '丢掉刚才的修改，按原样继续排队': 'Discard my changes and keep it queued as-is',
  '改好了，重新排队': 'Done editing, re-queue',
  '正在编辑，先不派发': 'editing — not dispatching yet',
  '正在编辑·先不派发': 'Editing · held',
  '已改好，重新排队。': 'edited and put back in line.',
  '改完，重新排队': 'edited, back in line',
  '已放弃修改，按原样排队。': 'edit discarded — kept in line exactly as it was.',
  '删除失败：': 'Delete failed: ',
  '放弃修改，按原样排队': 'edit discarded, kept in line as-is',
  '开始就地修改（先不派发）': 'opened for in-place editing (held)',
  '任务内容不能空。': 'Task text cannot be empty.',
  '请先选发送窗口。': 'Pick the target window first.',
  '删除这条': 'Delete this one',
  '不要这条了': 'Drop this task',
  '不改了': 'Never mind',
  '附件不变': 'Attachments unchanged',
  '有一条正在就地修改，先跳过它': 'one task is being edited in place — skipped for now',
  '正在修改': 'Editing',
  '已拿掉附件：': 'Attachment removed: ',
  '读队列失败：': 'Reading the queue failed: ',
  '未扫描到窗口，请确认 OpenClaw 网关正在运行。': 'No windows found. Make sure the OpenClaw gateway is running.',
  '已': '',
  '纳入': 'included',
  '移出': 'excluded',
  '派工范围。': ' from the dispatch scope.',
  '（手填，不用名单里的）': '(type it by hand)',
  '（没查到，请手填）': '(none found — type it by hand)',
  '查不到名单（可继续手填）': 'List unavailable (you can still type it)',
  '群': 'group',
  '个人': 'direct',
  '当前未开启。勾选后下次开机自动运行（只写当前用户的启动项，不动系统服务，随时可取消）。':
    'Currently off. Ticking it starts this program at next boot (writes only the current user\'s Run entry, touches no system service, you can undo anytime).',
  '正在发…': 'Sending…',
  '色值 —': 'color —',
  '）': ')',
  '派发失败、模型服务不通等。修复后点「重试」，不再需要可点「删除」。':
    'Dispatch failed, model service unreachable, etc. Fix it and press "Retry", or "Delete" if you do not need it anymore.',
  '选通道自动列出该通道的会话；名单不对时点「刷新地址」重查。':
    'Picking a channel lists its conversations; if the list looks wrong, press "Refresh list".',
  '放大查看': 'Zoom',
  '异常任务': 'Failed tasks',
  '窗口已回完、判为完成的任务（没收到回执的也在这里，标签会写明）。':
    'Tasks whose window has replied and are judged done (ones with no receipt are here too — the label says so).',
  '保存': 'Save',
  '已保存 ': 'Saved ',
  '设置已保存。': 'Settings saved.',
  '保存失败：': 'Save failed: ',
  '设置失败：': 'Saving settings failed: ',
  '读设置失败：': 'Reading settings failed: ',
  '导出 CSV': 'Export CSV',
  '导出范围': 'Export range',
  '最近 50 条': 'Last 50',
  '最近 200 条': 'Last 200',
  '最近 1000 条': 'Last 1000',
  '最近 1 天': 'Last 1 day',
  '最近 3 天': 'Last 3 days',
  '最近 7 天': 'Last 7 days',
  '最近 30 天': 'Last 30 days',
  '全部': 'All',
  '正在导出…': 'Exporting…',
  '未导出（已取消）': 'Not exported (cancelled)',
  '导出失败：': 'Export failed: ',
  '发送失败：': 'Sending failed: ',
  '已发送，请到收件处查看。': 'Sent — check the recipient.',

  /* ---- 日志页 ---- */
  '运行日志': 'Run log',
  '记录每次加入队列、派发、完成与通知。': 'Records every queue, dispatch, completion and notification.',
  '暂无记录。': 'Nothing recorded yet.',
  '读日志失败：': 'Reading the log failed: ',

  /* ---- 通道未配置提示 ---- */
  '未检测到可用的聊天通道': 'No usable chat channel found',
  '本机 OpenClaw 未启用任何聊天通道（如飞书、微信、Telegram）。任务结果可仅在本程序的日志与界面中查看，不对外发送。':
    'Your OpenClaw has no chat channel enabled (Feishu, WeChat, Telegram…). Task results stay in this program\'s log and interface only; nothing is sent out.',
  '不发送通知，仅用界面': 'No notifications, interface only',
  '我去配置，重新检测': 'I will configure it, re-check',
  '重新检测失败：': 'Re-check failed: ',

  /* ---- 状态与错误 ---- */
  '程序后台尚未就绪': 'Backend is not ready yet',
  '程序后台尚未就绪：': 'Backend is not ready yet: ',
  '操作失败：': 'Operation failed: ',
  '加入队列失败：': 'Queueing failed: ',
  '改勾选失败：': 'Changing the tick failed: ',
  '确认失败：': 'Confirmation failed: ',
  '继续失败：': 'Resuming failed: ',
  '取消失败：': 'Cancelling failed: ',
  '切换失败：': 'Switching failed: ',
  '读窗口失败：': 'Reading windows failed: ',
  '读取执行中任务失败：': 'Reading running tasks failed: ',
  '读状态失败：': 'Reading status failed: ',
  '选文件失败：': 'Picking files failed: ',
  '本次检查出错：': 'This check hit an error: ',
  '已检查一轮。': 'One check finished.',
  '正在检查一轮…': 'Running a check…',
  '已暂停自动派工。': 'Auto dispatch paused.',
  '已确认并发，将继续派发。': 'Concurrency confirmed, dispatching on.',
  '已暂不派发，任务保留在队列中。': 'Kept in the queue, nothing dispatched.',

  /* ---- 截图层 ---- */
  '第 1 步：拖动鼠标框出要截的区域': 'Step 1: drag to select the area to capture',
  '第 1 步：拖动鼠标框出要截的区域（放大镜显精确坐标，Esc 取消）':
    'Step 1: drag to select the area (the loupe shows exact coordinates, Esc cancels)',
  '第 2 步：标注（Ctrl+Z 撤销、Ctrl+Y 重做）；点「完成」送进待发区，不直接排队':
    'Step 2: annotate (Ctrl+Z undo, Ctrl+Y redo); press "Done" to move it into the outbox, it is not queued yet',
  '选区 ': 'Selection ',
  ' · 花圈/椭圆/方形/箭头/文字/马赛克/取色，Ctrl+Z 撤销，Ctrl+Y 重做，回车完成，Esc 取消':
    ' · freehand/ellipse/rect/arrow/text/mosaic/picker, Ctrl+Z undo, Ctrl+Y redo, Enter done, Esc cancel',
  '坐标 ': 'x,y ',
  '色值 ': 'color ',
  '按 Ctrl+C 复制色值': 'Ctrl+C copies the color',
  '完成（回车）': 'Done (Enter)',
  '取消（Esc）': 'Cancel (Esc)',
  '另存为：只存盘，不列入待发': 'Save As: save to a file only, do not add to the outbox',
  '抹掉所有标注': 'Clear all annotations',
  '撤销上一笔（Ctrl+Z）': 'Undo last (Ctrl+Z)',
  '重做（Ctrl+Y）': 'Redo (Ctrl+Y)',
  '花圈：按住鼠标自由画（截图后可直接画）': 'Freehand: hold and draw (you can draw right after capturing)',
  '椭圆：拖一个椭圆/圆圈': 'Ellipse: drag an ellipse/circle',
  '方形：拖一个方框': 'Rectangle: drag a box',
  '箭头：拖一条线，末端带箭头': 'Arrow: drag a line, arrowhead at the end',
  '文字：在选区内点一下开始打字，回车落定': 'Text: click inside the selection to type, Enter to place it',
  '马赛克：拿小圆圈涂抹，涂到哪哪就模糊': 'Mosaic: paint with the small circle, whatever it covers gets blurred',
  '取色器：点一下取屏幕上任何一点的颜色（Ctrl+C 复制色值）':
    'Color picker: click anywhere on screen to take its color (Ctrl+C copies the value)',
  '取色器刚取到的颜色': 'Color just picked',
  '细': 'Thin',
  '中': 'Medium',
  '粗': 'Bold',
  '底图解不开，截图取消。': 'The captured image could not be decoded; capture cancelled.',
  '截图失败：': 'Capture failed: ',
  '合成图片失败：': 'Composing the image failed: ',
  '存图失败：': 'Saving the image failed: ',
  '先用鼠标框出要截的区域。': 'Drag a region first.',
  '截图已放进待发区（和传文件一样）：先写要说的话，再点「加入队列」。':
    'The screenshot is in the outbox (same as attachments): write the message, then press "Queue it".',
  '这个点取不到颜色。': 'No color available at this point.',
  '色值已复制：': 'Color copied: ',
  '复制失败：': 'Copy failed: ',
  '图已存到：': 'Image saved to: ',
  '没存（你在对话框里取消了）': 'Not saved (you cancelled the dialog)',
  '已保存': 'Saved',
  '未导出': 'Not exported',
  '这个类型不能在这里放大看（图片才可以）。': 'This type cannot be enlarged here (only images can).',

  /* ---- B 阶段：Go 那边推给界面的运行消息（整句、不带变量的先登记）----
     带数字/窗口名的句子走下面的 RULES，窗口名与通道名不翻。 */
  '工头派工台已启动': 'Foreman started',
  '已暂停自动派发': 'Auto dispatch paused',
  '异常已处理，自动恢复派发': 'Error cleared, dispatch resumed automatically',
  '网关已恢复，自动恢复派发': 'Gateway back up, dispatch resumed automatically',
  '网关连接失败，本次不派发：': 'Gateway connection failed, skipping this round: ',
  '已停止派发：': 'Dispatch halted: ',
  '排队任务暂未派发：': 'Queued task not dispatched: ',
  '本次检查出错：': 'This check failed: ',
  '托盘起不来：': 'Tray unavailable: ',
  '（点右上角叉会直接退出）': ' (closing the window quits)',
  '目标窗口不在派工范围（或已不存在）': 'Target window is not in the dispatch scope (or no longer exists)',
  '目标窗口不存在或已归档': 'Target window no longer exists (archived or deleted)',
  '[dry] 条件满足，会准备关机（60 秒缓冲）': '[dry] conditions met, would start shutdown prep (60-second buffer)',
  '[dry] 缓冲结束，会执行关机': '[dry] buffer ended, would run the shutdown',
  '队列清空且全空闲，60 秒后自动关机（期间有新活动自动撤销）': 'Queue empty and all windows idle — auto shutdown in 60 seconds (any new activity cancels it)',
  '执行关机（shutdown /s /t 0 /f）': 'Running the shutdown (shutdown /s /t 0 /f)',
  '自动关机已撤销：': 'Auto shutdown cancelled: ',
  '不在设定的关机时段': 'outside the configured shutdown window',
  '程序重启，上次挂着的关机准备作废': 'the program restarted, so the pending shutdown prep was dropped',
  '等条件满足': 'waiting for conditions',
  '用户在界面点击取消': 'cancelled from the interface',
  '仍有任务未完成': 'tasks are still unfinished',
  '还有任务标着异常，没处理': 'some tasks are still marked as errors',
  '读任务数失败': 'reading task counts failed',
  '未勾选任何可派工窗口': 'no dispatchable window is selected',
  '模型服务还在忙': 'the model service is still busy',
  '仍在执行任务': ' is still running a task',
  '刚有新输入': ' just received new input',
  '关机命令失败: ': 'Shutdown command failed: ',
  '撤销：': 'Cancelled: ',
  /* ---- C 阶段：日志页里的固定说法（日志是已存的历史，Go 不改，只在显示层翻）---- */
  '未配置通知通道，已跳过发送': 'No notification channel configured, sending skipped',
  '重连后找不到通道地址，通知未发: ': 'Channel address not found after reconnect, notification not sent: ',
  '找不到通道地址，通知未发: ': 'Channel address not found, notification not sent: ',
  '网关不可用，通知未发: ': 'Gateway unavailable, notification not sent: ',
  '发送失败（已重试 ': 'Send failed (retried ',
  ' 次）': ' time(s))',
  '队列清空且全空闲，执行关机': 'Queue empty and all windows idle — running the shutdown',
  '这台机器没探测到聊天通道，用户选了「不用通知，只用界面」': 'No chat channel detected on this machine; the user chose "no notifications, interface only"',
  '重新排队': 'Requeued',
  /* 托盘那几条走 feed，同样登记整句（不登记会被短词条「图」换成 img、「中」换成 Medium） */
  '已在任务栏图标区常驻：左键叫回主界面，右键出菜单': 'Living in the tray now: left-click brings the window back, right-click opens the menu',
  '托盘图标读不出来（窗口与功能照常，只是任务栏里没图标）': 'Tray icon could not be loaded (windows and features work fine, there is just no tray icon)',
  '托盘起不来：建消息窗失败（点右上角叉会直接退出，不会藏窗口）': 'Tray unavailable: could not create the message window (closing the window quits instead of hiding it)',
  '不发通知（只记日志）': 'No notification channel (log only)',
  '这台机器探不到服务状态，只看窗口': 'This machine cannot probe the service state, windows only',
};

/* 按中文长度从长到短排好，避免短句先抢走长句的一部分。 */
const PAIRS = Object.keys(DICT)
  .sort((a, b) => b.length - a.length)
  .map((zh) => [zh, DICT[zh]]);

/* 带数字、窗口名、通道名的运行消息：语序跟中文不一样，整句换成英文模板。
   只换句子本身，抓出来的窗口名/通道名/报错原文一律保持原文（那是 OpenClaw 那边的数据）。
   规则先跑，再走上面的片段词典（不先跑规则，片段会把句子切碎）。 */
// 规则抓到的数据段先藏成占位符，片段词典翻完再放回——
// 不藏的话「飞书群」会被短词条「群」换成「飞书group」。
let stash = [];
function keep(x) {
  stash.push([x, false]);
  return '\u0001' + (stash.length - 1) + '\u0001';
}
// keepText：括号里的原因（「N 条任务均为异常」「gateway: 报错原文」）要翻，但不能被短词条切碎
// 所以先藏起来；放回时**只走整句规则**（不走片段词典）——这样「3 条任务均为异常」能整句换英文
// 而任意中文不会被「中」→Medium 这种短词条换坏。
function keepText(x) {
  stash.push([x, true]);
  return '\u0001' + (stash.length - 1) + '\u0001';
}
function innerTr(s) {
  for (const [re, fn] of RULES) {
    if (re.test(s)) s = s.replace(re, fn);
  }
  return unkeep(s, 2);
}
function unkeep(s, depth) {
  if (depth > 3) return s;
  return s.replace(/\u0001(\d+)\u0001/g, (m, i) => {
    const e = stash[Number(i)];
    if (e === undefined) return m;
    return e[1] ? innerTr(e[0]) : e[0];
  });
}
const RULES = [
  [/^T-(\d{3}) 已派发给 (.+)$/, (m, id, w) => `T-${id} dispatched to ${keep(w)}`],
  [/^T-(\d{3}) 已完成并收到回执 ✅$/, (m, id) => `T-${id} done, completion receipt received ✅`],
  [/^T-(\d{3}) 已完成（依据窗口状态判定，未收到回执）$/, (m, id) => `T-${id} done (judged from window state, no receipt)`],
  [/^T-(\d{3}) 目标窗口已空闲，开始 (\d+) 秒复核$/, (m, id, n) => `T-${id} target window is idle, starting a ${n}-second recheck`],
  [/^\[dry\] T-(\d{3}) 判定完成（完成回执=(.*?)）$/, (m, id, a) => `[dry] T-${id} judged done (receipt=${a})`],
  [/^T-(\d{3}) 结果已由 (.+?) 通道送达，不重复转发$/, (m, id, ch) => `T-${id} result already delivered via the ${keep(ch)} channel, not forwarding again`],
  [/^\[试运行\] 将把 T-(\d{3}) 派发给 (.+)$/, (m, id, w) => `[dry run] would dispatch T-${id} to ${keep(w)}`],
  [/^T-(\d{3}) 待派发，但目标窗口与执行中的任务使用同一模型服务，已挂起等待确认$/, (m, id) => `T-${id} is held for confirmation: its target window shares a model service with a task already running`],
  [/^派发已暂停（(.*?)），本次跳过$/, (m, x) => `Dispatch paused (${keepText(x)}), skipping this round`],
  [/^派发已停止（(.*?)），等待处理$/, (m, x) => `Dispatch halted (${keepText(x)}), waiting`],
  [/^已开始跟踪窗口，每 (\d+) 秒检查一次$/, (m, n) => `Watching windows now, checking every ${n} seconds`],
  [/^已检查一轮：派出去 (\d+) 条/, (m, n) => `Checked one round: ${n} sent`],
  [/，完成 (\d+) 条/, (m, n) => `, ${n} done`],
  [/（派发已停止：/, () => ' (dispatch halted: '],
  [/（派发已暂停：/, () => ' (dispatch paused: '],
  [/同时已派满 (\d+) 个窗口/, (m, n) => `all ${n} window slot(s) already in use`],
  [/显卡正忙（利用率 (\d+)%，连续 (\d+) 轮超过 (\d+)%）/, (m, p, n, t) => `the GPU is busy (${p}% utilization, ${n} consecutive rounds above ${t}%)`],
  [/模型服务还在忙/, () => 'the model service is still busy'],
  [/窗口 (.*?) 用的模型服务正忙，这条先排队/, (m, w) => `the model service used by window ${keep(w)} is busy, this one stays queued`],
  [/窗口 (.*?) 正忙（还有 (\d+) 条没处理完）/, (m, w, n) => `window ${keep(w)} is busy (${n} task(s) still unfinished)`],
  [/窗口 (.*?) 刚收到消息，本轮先避开/, (m, w) => `window ${keep(w)} just received input, skipped this round`],
  [/窗口 (.*?) 仍在执行任务/, (m, w) => `window ${keep(w)} is still running a task`],
  [/窗口 (.*?) 刚有新输入/, (m, w) => `window ${keep(w)} just received new input`],
  [/^准备关机，还剩 (\d+) 秒$/, (m, n) => `Shutting down soon, ${n} seconds left`],
  [/(\d+) 条任务均为异常/, (m, n) => `all ${n} task(s) are in error`],
  [/队列中 (\d+) 条任务均为异常，已停止派发，等待处理/, (m, n) => `all ${n} queued task(s) are in error, dispatch halted, waiting`],
];

/* 翻一段文字：英文界面下先按整句规则换（带数字/窗口名的句子），再按词典片段换。
   中文界面原样返回。窗口名、通道名、任务正文不在词典里，所以保持原文。 */
function tr(s) {
  if (uiLang !== 'en' || s == null) return s;
  let out = String(s);
  stash = [];
  for (const [re, fn] of RULES) {
    if (re.test(out)) out = out.replace(re, fn);
  }
  for (const [zh, en] of PAIRS) {
    if (out.indexOf(zh) >= 0) out = out.split(zh).join(en);
  }
  return unkeep(out, 0);
}

/* 静态页面（index.html 里的文字、悬停说明、输入框提示）整页换一遍。
   原文先存进 data-zh-*，切回中文时还原（不存就回不去了）。 */
const ATTRS = ['title', 'placeholder', 'aria-label', 'alt'];
const HAS_CJK = /[\u4e00-\u9fff]/;
function walkDom(node) {
  if (node.nodeType === 3) {
    const t = node.nodeValue;
    if (uiLang === 'en') {
      if (HAS_CJK.test(t)) {
        node.parentNode.setAttribute('data-zh-text', (node.parentNode.getAttribute('data-zh-text') || '') + t);
        node.nodeValue = tr(t);
      }
    } else if (t && !HAS_CJK.test(t)) {
      const keep = node.parentNode.getAttribute('data-zh-text');
      if (keep) { node.nodeValue = keep; node.parentNode.removeAttribute('data-zh-text'); }
    }
    return;
  }
  if (node.nodeType !== 1) return;
  if (node.hasAttribute && node.hasAttribute('data-no-i18n')) return; // 语言下拉自己：不许被翻译（不然「中文」会被换成 Medium）
  for (const a of ATTRS) {
    const v = node.getAttribute(a);
    if (!v) continue;
    if (uiLang === 'en') {
      if (HAS_CJK.test(v)) {
        node.setAttribute('data-zh-' + a, v);
        node.setAttribute(a, tr(v));
      }
    } else {
      const keep = node.getAttribute('data-zh-' + a);
      if (keep) { node.setAttribute(a, keep); node.removeAttribute('data-zh-' + a); }
    }
  }
  for (let c = node.firstChild; c; c = c.nextSibling) walkDom(c);
}
function applyDom() {
  walkDom(document.getElementById('app') || document.body);
  const t = document.querySelector('title');
  if (t) t.textContent = uiLang === 'en' ? 'Foreman CatHerder' : t.getAttribute('data-zh-text') || '工头派工台 CatHerder';
  if (t) t.setAttribute('data-zh-text', '工头派工台 CatHerder');
  document.documentElement.lang = uiLang === 'en' ? 'en' : 'zh-CN';
  // 英文界面下挂一条固定提示：任务正文/窗口名/日志/通道名来自 OpenClaw，不翻。
  const ln = document.getElementById('lang-note');
  if (ln) ln.classList.toggle('hidden', uiLang !== 'en');
}

/* 日志页（C 阶段）：日志是已经存进库的历史，Go 那边不改，只在显示层翻。
   行格式：`时间 [类别] #任务号 正文`——时间/任务号原样，类别查表，正文走 RULES+词典。
   窗口名、通道名、报错原文、文件路径不在词典里 → 保持原文。 */
const LOG_KINDS = {
  add: 'queued', dispatch: 'dispatched', done: 'done', error: 'error', cancel: 'deleted',
  retry: 'requeued', pause: 'paused', halt: 'halted', notify: 'notify', confirm: 'confirm',
  edit: 'edit',
  attach: 'attachment', shot: 'screenshot', export: 'export', shutdown: 'shutdown',
  debug: 'debug', keep: 'keep',
};
const LOG_RULES = [
  // 队列异常停止这条日志里写的是全句（feed 里只写一半），单独登记整句，免得被片段切碎。
  [/^队列中 (\d+) 条任务均为异常，已停止派发，等待处理$/, (m, n) => `Queue: all ${n} task(s) are in error — dispatch halted, waiting`],
  [/^模型服务不通: (.*)$/, (m, e) => `Model service unreachable: ${keep(e)}`],
  [/^T-(\d{3}) 已附 (\d+) 个文件：(.*)$/, (m, id, n, f) => `T-${id}: ${n} file(s) attached: ${keep(f)}`],
  [/^截图已存 (.*?)（(\d+) KB）$/, (m, f, n) => `Screenshot saved ${keep(f)} (${n} KB)`],
  [/^截图另存为 (.*?)（(\d+) KB）$/, (m, f, n) => `Screenshot saved as ${keep(f)} (${n} KB)`],
  [/^T-(\d{3}) (已确认并发|暂不派发，保留队列)（不再提醒=(.*?)）$/, (m, id, v, mute) => `T-${id} ${v === '已确认并发' ? 'confirmed concurrent dispatch' : 'declined, kept in queue'} (no more prompts=${mute})`],
  [/^T-(\d{3}) 读回复失败: (.*)$/, (m, id, e) => `T-${id} reading the reply failed: ${keep(e)}`],
  [/^T-(\d{3}) 已完成但读取不到回复内容（可能位于上下文压缩事件中）$/, (m, id) => `T-${id} done but the reply text is unreadable (possibly inside a context-compaction event)`],
  [/^T-(\d{3}) 的附件 (.*?) 读不到（(.*?)），这个不发了$/, (m, id, f, e) => `T-${id} attachment ${keep(f)} unreadable (${keep(e)}), not sending it`],
  [/^导出 (\d+) 条 → (.*)$/, (m, n, p) => `Exported ${n} row(s) → ${keep(p)}`],
  [/^加入队列 → (.*)$/, (m, w) => `Added to queue → ${keep(w)}`],
  [/^出错: (.*)$/, (m, e) => `Error: ${keep(e)}`],
];
function trLog(line) {
  if (uiLang !== 'en' || line == null) return line;
  const m = String(line).match(/^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) \[([^\]]*)\] #(\d+) ?([\s\S]*)$/);
  if (!m) return tr(line);
  const [, at, kind, tid, body] = m;
  if (body.indexOf('[前端]') === 0) return line; // 界面自检那类调试行：内部文字，原样不翻
  stash = [];
  let b = body;
  for (const [re, fn] of LOG_RULES.concat(RULES)) {
    if (re.test(b)) b = b.replace(re, fn);
  }
  for (const [zh, en] of PAIRS) {
    if (b.indexOf(zh) >= 0) b = b.split(zh).join(en);
  }
  return at + ' [' + (LOG_KINDS[kind] || kind) + '] #' + tid + (b ? ' ' + unkeep(b, 0) : '');
}

/* 切语言：改状态 → 重画静态页面。动态内容由各渲染函数下次刷新时自然带上。 */
function setLang(v) {
  const next = v === 'en' ? 'en' : 'zh';
  if (next === uiLang) return;
  uiLang = next;
  applyDom();
}

/* 给 shot.js 用：它自己有一份静态工具条，切语言时要一起换。 */
window.__i18n = {
  tr,
  trLog,
  setLang,
  getLang: () => uiLang,
  applyDom,
};
