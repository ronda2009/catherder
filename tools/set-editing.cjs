// 把某条排队任务直接置成「正在改」/改回「排队」——用来验「正在改的这条不派发、后面的照常派」。
// 界面里这一步是点「编辑」，命令行没有对应命令，所以直接改库（只动这一条的状态列）。
// 用法：node tools\set-editing.cjs --id 35 --on | --off
const { DatabaseSync } = require('node:sqlite');
const args = process.argv.slice(2);
const id = Number(args[args.indexOf('--id') + 1]);
const on = args.includes('--on');
if (!id) { console.log('要给 --id 和 --on/--off'); process.exit(1); }
const p = process.env.APPDATA + '\\catherder\\catherder.sqlite';
const db = new DatabaseSync(p);
if (on) {
  const r = db.prepare(`UPDATE tasks SET state='editing', editing_at=strftime('%s','now')*1000 WHERE id=? AND state='queued'`).run(id);
  console.log('置为正在改：改了 ' + r.changes + ' 行');
} else {
  const r = db.prepare(`UPDATE tasks SET state='queued', editing_at=0 WHERE id=? AND state='editing'`).run(id);
  console.log('改回排队：改了 ' + r.changes + ' 行');
}
const rows = db.prepare(`SELECT id,state,body FROM tasks WHERE state IN ('queued','editing') ORDER BY prio,id`).all();
for (const r of rows) console.log(`T-${String(r.id).padStart(3, '0')} [${r.state}] ${r.body}`);
db.close();
