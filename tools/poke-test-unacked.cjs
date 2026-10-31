// 给测试版库造一条「本轮完成、但没收到回执、也没送达通知」的任务数据：
// 挑一条 done_unacked，把 done_at 改成现在、notified_at 清空 —— 用来验「未回执也进已完成区」。
// 用法：node tools\poke-test-unacked.cjs [任务id]
const { DatabaseSync } = require('node:sqlite');
const p = 'C:\\Users\\Administrator\\AppData\\Roaming\\catherder\\catherder.sqlite';
const want = Number(process.argv[2] || 0);
const db = new DatabaseSync(p);
const now = Date.now();
const row = db.prepare(`SELECT id,state,ack,notified_at FROM tasks WHERE state='done_unacked' ${want ? 'AND id=?' : ''} ORDER BY id DESC LIMIT 1`).get(...(want ? [want] : []));
if (!row) { console.log('测试库里没有 done_unacked 任务'); db.close(); process.exit(1); }
db.prepare('UPDATE tasks SET done_at=?, notified_at=NULL WHERE id=?').run(now, row.id);
const after = db.prepare('SELECT id,state,ack,done_at,notified_at FROM tasks WHERE id=?').get(row.id);
console.log(`#${after.id} state=[${after.state}] ack=${after.ack} done_at=${new Date(after.done_at).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false })} notified_at=${after.notified_at ?? '(空)'}`);
db.close();
