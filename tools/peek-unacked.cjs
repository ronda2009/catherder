// 查正式版库里 done_unacked / dispatched 任务的 notified_at、ack、note（判「为什么没跳进已完成」）。
// 用法：node tools\peek-unacked.cjs
const { DatabaseSync } = require('node:sqlite');
const p = 'C:\\Users\\Administrator\\Desktop\\墨云(OpenClaw)\\工头派工台 正式版\\catherder.sqlite';
const db = new DatabaseSync(p, { readOnly: true });
const f = (ms) => (!ms ? '(0/无)' : new Date(Number(ms)).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false }));
const s = db.prepare("SELECT key,value FROM settings WHERE key IN ('notify_channel','ui_lang')").all();
console.log('设置: ' + s.map((x) => `${x.key}=[${x.value}]`).join('  '));
const rows = db.prepare(`SELECT id,state,target_label,ack,notified_at,done_at,dispatched_at,note,body FROM tasks
  WHERE state IN ('done','done_unacked','dispatched') ORDER BY id DESC LIMIT 12`).all().reverse();
console.log('任务（done/done_unacked/dispatched，最近 12 条）：');
for (const r of rows) {
  console.log(`#${r.id} state=[${r.state}] ack=${r.ack} notified_at=${f(r.notified_at)}`);
  console.log(`   → ${r.target_label}  派发 ${f(r.dispatched_at)} 完成 ${f(r.done_at)} note=[${r.note || ''}]`);
  console.log(`   正文=[${String(r.body).slice(0, 50)}]`);
}
db.close();
