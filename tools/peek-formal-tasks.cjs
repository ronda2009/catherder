// 查**桌面正式版**库里的任务（正文/状态/目标窗口/时间），用于核对有没有把老板的单误撤/误加。
// 用法：node tools\peek-formal-tasks.cjs            # 最近 10 条
//      node tools\peek-formal-tasks.cjs --grep 整理记忆
const { DatabaseSync } = require('node:sqlite');
const p = 'C:\\Users\\Administrator\\Desktop\\墨云(OpenClaw)\\工头派工台 正式版\\catherder.sqlite';
const a = process.argv.slice(2);
const grep = a.includes('--grep') ? a[a.indexOf('--grep') + 1] : '';
const db = new DatabaseSync(p, { readOnly: true });
const f = (ms) => (!ms ? '(无)' : new Date(Number(ms)).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false }));
const sql = `SELECT id,state,target_key,target_label,body,created_at,dispatched_at,done_at FROM tasks
  ${grep ? "WHERE body LIKE ?" : ''} ORDER BY id DESC LIMIT 10`;
const rows = (grep ? db.prepare(sql).all('%' + grep + '%') : db.prepare(sql).all()).reverse();
console.log('正式版库 ' + (grep ? `含「${grep}」` : '最近 10 条') + '：' + rows.length + ' 条');
for (const r of rows) {
  console.log(`#${r.id} [${r.state}] → ${r.target_label}`);
  console.log(`   正文=[${String(r.body).slice(0, 60)}]`);
  console.log(`   提交 ${f(r.created_at)} 派发 ${f(r.dispatched_at)} 完成 ${f(r.done_at)}`);
}
db.close();
