// 验日志行的英文翻译（C 阶段）：拿真实日志行喂 trLog，看句子翻没翻、数据段保没保持原文。
// 判据：把「本该保持原文」的数据段（窗口名/路径/文件名/报错原文）从译文里删掉，
// 剩下的部分不该再有中文。
const fs = require('fs');
const path = require('path');
const dist = path.join(__dirname, '..', 'app', 'frontend', 'dist');
let src = fs.readFileSync(path.join(dist, 'i18n.js'), 'utf8').replace('window.__i18n', 'global.__i18n');
global.document = { getElementById: () => null, querySelector: () => null, body: {}, documentElement: {} };
eval(src);
const T = global.__i18n;
T.setLang('en');

// 真实日志行（库里格式：时间 [类别] #任务号 正文）
const lines = [
  '2026-10-27 00:05:44 [debug] #0 派发已暂停（调序自检中），本次跳过',
  '2026-10-27 00:05:45 [dispatch] #25 ',
  '2026-10-27 00:05:45 [done] #25 ack=true',
  '2026-10-27 00:05:45 [add] #26 加入队列 → feishu:g-墨云',
  '2026-10-27 00:05:45 [cancel] #25 ',
  '2026-10-27 00:05:45 [retry] #25 重新排队',
  '2026-10-27 00:05:45 [error] #25 模型服务不通: dial tcp refused',
  '2026-10-27 00:05:45 [debug] #0 排队任务暂未派发：显卡正忙（利用率 84%，连续 2 轮超过 30%）',
  '2026-10-27 00:05:45 [debug] #0 显卡正忙（利用率 84%，连续 2 轮超过 30%）',
  // 凌晨改的「就地编辑」与「调序」写进库的真日志行（验英文界面下会不会露中文）。
  '2026-10-27 00:05:45 [edit] #35 开始就地修改（先不派发）',
  '2026-10-27 00:05:45 [edit] #35 改完，重新排队',
  '2026-10-27 00:05:45 [edit] #35 放弃修改，按原样排队',
  '2026-10-27 00:05:45 [debug] #0 排队任务暂未派发：有一条正在就地修改，先跳过它',
  '2026-10-27 00:05:45 [debug] #0 排队任务暂未派发：目标窗口不在派工范围（或已不存在）',
  '2026-10-27 00:05:45 [halt] #0 队列中 5 条任务均为异常，已停止派发，等待处理',
  '2026-10-27 00:05:45 [notify] #0 未配置通知通道，已跳过发送',
  '2026-10-27 00:05:45 [notify] #0 发送失败（已重试 3 次）',
  '2026-10-27 00:05:45 [attach] #25 T-025 已附 2 个文件：a.png、b.txt',
  '2026-10-27 00:05:45 [shot] #0 截图已存 截图-20261026-235905.png（88 KB）',
  '2026-10-27 00:05:45 [export] #0 导出 12 条 → 工头派工台记录-20261026-2300.csv',
  '2026-10-27 00:05:45 [confirm] #25 T-025 已确认并发（不再提醒=）',
  '2026-10-27 00:05:45 [shutdown] #0 队列清空且全空闲，执行关机',
  '2026-10-27 00:05:45 [shutdown] #0 撤销：不在设定的关机时段',
  '2026-10-27 00:05:45 [shutdown] #0 撤销：程序重启，上次挂着的关机准备作废',
  '2026-10-27 00:05:45 [debug] #0 执行关机（shutdown /s /t 0 /f）',
  '2026-10-27 00:05:45 [debug] #0 已在任务栏图标区常驻：左键叫回主界面，右键出菜单',
  '2026-10-27 00:05:45 [debug] #0 已开始跟踪窗口，每 60 秒检查一次',
  '2026-10-27 00:05:45 [debug] #0 工头派工台已启动',
  '2026-10-27 00:05:45 [debug] #0 [前端] [截图自检] 自检完成',
  '2026-10-27 00:05:45 [pause] #0 派发已暂停（gateway: dial tcp 127.0.0.1:8080 refused），本次跳过',
];
// 这些是「OpenClaw 那边/用户侧」的数据，译文里必须原样出现，判残留前先删掉
const KEEP_AS_IS = ['feishu:g-墨云', '截图-20261026-235905.png', '工头派工台记录-20261026-2300.csv',
  '调序自检中', 'dial tcp refused', 'gateway: dial tcp 127.0.0.1:8080 refused', 'a.png、b.txt'];

let bad = 0;
for (const l of lines) {
  let out = T.trLog(l);
  let check = out;
  for (const k of KEEP_AS_IS) check = check.split(k).join('');
  const cjk = /[\u4e00-\u9fff]/.test(check);
  if (cjk) bad++;
  console.log((cjk ? '残留 ' : 'OK   ') + out.slice(0, 150));
}
console.log('--- 句子本身残留中文的行数 = ' + bad);
T.setLang('zh');
console.log('中文界面原样: ' + (T.trLog(lines[0]) === lines[0] ? 'OK' : '错'));
