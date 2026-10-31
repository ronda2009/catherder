// 读设置表（排查 paused 到底存的是什么）。临时用。
const { DatabaseSync } = require('node:sqlite');
const p = process.env.APPDATA + '\\catherder\\catherder.sqlite';
const db = new DatabaseSync(p, { readOnly: true });
const rows = db.prepare('SELECT key, value FROM settings').all();
for (const r of rows) console.log(r.key + ' = [' + r.value + ']  (len=' + String(r.value).length + ')');
db.close();
