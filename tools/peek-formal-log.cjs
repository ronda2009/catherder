// 查**正式版库**的事件日志（表名是 events，不是 logs；列是 at/task_id/kind/detail）。
// 用法：node tools\peek-formal-log.cjs T-022
//      node tools\peek-formal-log.cjs --kind forward
//      node tools\peek-formal-log.cjs --from "2026-10-27 18:00"
const { DatabaseSync } = require('node:sqlite');
const p = 'C:\\Users\\Administrator\\Desktop\\墨云(OpenClaw)\\工头派工台 正式版\\catherder.sqlite';
const a = process.argv.slice(2);
const grep = a.length && !a[0].startsWith('--') ? a[0] : '';
const kind = a.includes('--kind') ? a[a.indexOf('--kind') + 1] : '';
const from = a.includes('--from') ? a[a.indexOf('--from') + 1] : '';
const db = new DatabaseSync(p, { readOnly: true });
const where = [];
const args = [];
if (grep) { where.push('detail LIKE ?'); args.push('%' + grep + '%'); }
if (kind) { where.push('kind=?'); args.push(kind); }
if (from) { where.push('at >= ?'); args.push(from); }
const rows = db.prepare(`SELECT at,kind,task_id,detail FROM events ${where.length ? 'WHERE ' + where.join(' AND ') : ''} ORDER BY at DESC LIMIT 25`).all(...args).reverse();
console.log(`正式版事件 命中 ${rows.length} 条${grep ? `（含「${grep}」）` : ''}${kind ? `（kind=${kind}）` : ''}${from ? `（${from} 起）` : ''}`);
for (const r of rows) console.log(`${r.at} [${r.kind}] #${r.task_id ?? '-'} ${String(r.detail).slice(0, 130)}`);
db.close();
