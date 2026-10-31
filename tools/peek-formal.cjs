// 查正式版库：tasks 表有没有新列（editing_at）、设置里有没有旧值残留。
// 用法：node tools\peek-formal.cjs [库路径]
const { DatabaseSync } = require('node:sqlite');
const p = process.argv[2] || 'C:\\Users\\Administrator\\Desktop\\墨云(OpenClaw)\\工头派工台 正式版\\catherder.sqlite';
const db = new DatabaseSync(p, { readOnly: true });
const cols = db.prepare(`SELECT name FROM pragma_table_info('tasks')`).all().map((r) => r.name);
console.log('库: ' + p);
console.log('tasks 列: ' + cols.join(','));
console.log('有 editing_at: ' + (cols.includes('editing_at') ? '是（库已升到新版结构）' : '否'));
for (const r of db.prepare(`SELECT key,value FROM settings WHERE key IN ('paused','halt','max_parallel','gate_service','ui_lang','recheck_seconds')`).all()) {
  console.log(r.key + ' = [' + r.value + ']  (len=' + String(r.value).length + ')');
}
const st = db.prepare(`SELECT state, COUNT(*) n FROM tasks GROUP BY state`).all();
console.log('任务计数: ' + st.map((r) => r.state + '=' + r.n).join(' '));
const q = db.prepare(`SELECT id,state,prio,target_label FROM tasks WHERE state IN ('queued','editing') ORDER BY prio,id`).all();
for (const r of q) console.log(`T-${String(r.id).padStart(3, '0')} [${r.state}] prio=${r.prio} → ${r.target_label}`);
db.close();
