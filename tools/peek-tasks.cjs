// 查测试版库里指定编号的任务（正文/状态/目标窗口/时间），用于核对误撤的测试任务。
// 用法：node tools\peek-tasks.cjs 56 57 58
const { DatabaseSync } = require('node:sqlite');
const db = new DatabaseSync(process.env.APPDATA + '\\catherder\\catherder.sqlite', { readOnly: true });
const ids = process.argv.slice(2).map(Number).filter(Boolean);
const f = (ms) => (!ms ? '(无)' : new Date(Number(ms)).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false }));
const where = ids.length ? 'WHERE id IN (' + ids.map(() => '?').join(',') + ')' : 'WHERE id >= (SELECT MAX(id)-8 FROM tasks)';
for (const r of db.prepare(`SELECT id,state,target_key,target_label,body,note,created_at,dispatched_at,done_at
  FROM tasks ${where} ORDER BY id`).all(...ids)) {
  console.log(`#${r.id} [${r.state}] → ${r.target_label} (${r.target_key})`);
  console.log(`   正文=[${r.body}]`);
  console.log(`   提交 ${f(r.created_at)} 派发 ${f(r.dispatched_at)} 完成 ${f(r.done_at)} 备注=[${r.note || ''}]`);
}
db.close();
