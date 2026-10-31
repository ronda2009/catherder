// 看 OpenClaw 会话库里哪些表/字段能当「窗口还在干活」的信号（查误判闲置用）。
// 用法：node tools\peek-openclaw-db.cjs            → 列表名
//      node tools\peek-openclaw-db.cjs 表名        → 列该表字段
const { DatabaseSync } = require('node:sqlite');
const p = 'C:\\Users\\Administrator\\.openclaw\\agents\\main\\agent\\openclaw-agent.sqlite';
const db = new DatabaseSync(p, { readOnly: true });
const want = process.argv[2];
if (!want) {
  const t = db.prepare(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`).all().map((r) => r.name);
  console.log('表 (' + t.length + '): ' + t.join(', '));
} else {
  for (const name of want.split(',')) {
    const c = db.prepare(`SELECT name FROM pragma_table_info(?)`).all(name.trim()).map((r) => r.name);
    console.log(name.trim() + ' 列: ' + c.join(', '));
  }
}
db.close();
