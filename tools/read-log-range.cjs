// 按时间段倒派工台事件日志（查「昨晚那一刻派工台到底看到了什么」）。
// 用法：node tools\read-log-range.cjs --from "2026-10-26 19:40" --to "2026-10-26 21:15" [库路径]
// 时间写到分或到秒都行（"19:40" / "19:40:30"）；不传库路径读测试版那份，查正式版传桌面那份的 catherder.sqlite。
const { DatabaseSync } = require('node:sqlite');
const a = process.argv.slice(2);
const arg = (k) => a[a.indexOf(k) + 1];
const from = arg('--from'), to = arg('--to');
if (!from || !to) { console.log('要给 --from 和 --to（格式 2026-10-26 19:40 或 19:40:30）'); process.exit(1); }
// 秒可省：只写到分就自己补 :00（原来写死补一段，给到秒就拼成非法时间、查出 0 条——探测脚本自己错了）。
const ms = (s) => {
  const t = s.trim().replace(' ', 'T');
  const full = t.split(':').length >= 3 ? t : t + ':00';
  return new Date(full + '+08:00').getTime();
};
const p = a[a.length - 1].endsWith('.sqlite') ? a[a.length - 1] : process.env.APPDATA + '\\catherder\\catherder.sqlite';
const db = new DatabaseSync(p, { readOnly: true });
const t0 = ms(from), t1 = ms(to);
if (!Number.isFinite(t0) || !Number.isFinite(t1)) {
  console.log('时间格式不对（用 2026-10-26 19:40 或 2026-10-26 19:40:30）');
  process.exit(1);
}
const rows = db.prepare(`SELECT at,kind,task_id,detail FROM events WHERE at>=? AND at<=? ORDER BY at LIMIT 400`)
  .all(t0, t1);
const f = (ms) => new Date(ms).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false });
console.log('库: ' + p);
console.log('区间 ' + from + ' ~ ' + to + ' 共 ' + rows.length + ' 条');
for (const r of rows) console.log(f(r.at) + ' [' + r.kind + '] #' + r.task_id + ' ' + r.detail);
db.close();
