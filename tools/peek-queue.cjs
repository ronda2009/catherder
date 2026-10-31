// 读测试版库里「排队中」的顺序（验证上移/下移真的写进 prio）。临时排查用。
const { DatabaseSync } = require('node:sqlite');
const path = process.argv[2] || (process.env.APPDATA + '\\catherder\\catherder.sqlite');
const db = new DatabaseSync(path, { readOnly: true });
const rows = db.prepare(`SELECT id, state, prio, target_label, substr(body,1,18) AS b
  FROM tasks WHERE state IN ('queued','paused') ORDER BY prio, id`).all();
console.log('库: ' + path);
console.log('排队中 ' + rows.length + ' 条（按 prio 排＝派发顺序）');
for (const r of rows) {
  console.log('  T-' + String(r.id).padStart(3, '0') + '  prio=' + r.prio + '  ' + r.state + '  → ' + r.target_label + '  | ' + r.b);
}
const c = db.prepare(`SELECT kind, count(*) AS n FROM events WHERE kind='debug' AND at >= ? GROUP BY kind`).all(Date.now() - 40 * 60000);
console.log('近 40 分钟 debug 事件: ' + JSON.stringify(c));
const d = db.prepare(`SELECT at, detail FROM events WHERE kind='debug' AND detail LIKE '%顺序%' ORDER BY id DESC LIMIT 4`).all();
for (const r of d) console.log(new Date(Number(r.at)).toLocaleTimeString('zh-CN') + ' ' + r.detail);
db.close();
