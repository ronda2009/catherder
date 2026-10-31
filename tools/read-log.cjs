// 读派工台自己的事件日志（表 events）。用法: node tools\read-log.cjs [匹配文字] [分钟数] [库路径]
// 不传库路径就读测试版那份；正式版在桌面「工头派工台 正式版\catherder.sqlite」，查正式版干过什么要传它。
const { DatabaseSync } = require('node:sqlite');
const fs = require('node:fs');
const dbPath = process.argv[4] || (process.env.APPDATA + '\\catherder\\catherder.sqlite');
if (!fs.existsSync(dbPath)) { console.log('库不存在:', dbPath); process.exit(0); }
const like = '%' + (process.argv[2] || '自检') + '%';
const mins = Number(process.argv[3]) || 15;
const db = new DatabaseSync(dbPath, { readOnly: true });
const rows = db.prepare(
  `SELECT at, kind, detail FROM events WHERE detail LIKE ? AND at >= ? ORDER BY id DESC LIMIT 16`
).all(like, Date.now() - mins * 60 * 1000);
if (!rows.length) console.log(`近 ${mins} 分钟没有含「${process.argv[2] || '自检'}」的日志`);
for (const r of rows.reverse()) {
  console.log(new Date(Number(r.at)).toLocaleTimeString('zh-CN') + ' [' + r.kind + '] ' + r.detail);
}
db.close();
