// 查**测试版库**的事件（表 events：at/task_id/kind/detail），按关键字/时间过滤。
// 用法：node tools\peek-test-log.cjs 显卡
//      node tools\peek-test-log.cjs --kind dispatch
const { DatabaseSync } = require('node:sqlite');
const p = 'C:\\Users\\Administrator\\AppData\\Roaming\\catherder\\catherder.sqlite';
const a = process.argv.slice(2);
const grep = a.length && !a[0].startsWith('--') ? a[0] : '';
const kind = a.includes('--kind') ? a[a.indexOf('--kind') + 1] : '';
const db = new DatabaseSync(p, { readOnly: true });
const where = [], args = [];
if (grep) { where.push('detail LIKE ?'); args.push('%' + grep + '%'); }
if (kind) { where.push('kind=?'); args.push(kind); }
const rows = db.prepare(`SELECT at,kind,task_id,detail FROM events ${where.length ? 'WHERE ' + where.join(' AND ') : ''} ORDER BY at DESC LIMIT 12`).all(...args).reverse();
console.log(`测试版事件 命中 ${rows.length} 条${grep ? `（含「${grep}」）` : ''}${kind ? `（kind=${kind}）` : ''}`);
for (const r of rows) console.log(`${new Date(Number(r.at)).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false })} [${r.kind}] ${String(r.detail).slice(0, 120)}`);
db.close();
