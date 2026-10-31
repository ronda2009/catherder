// 列正式版库里的表名（写脚本时别猜表名）。
const { DatabaseSync } = require('node:sqlite');
const db = new DatabaseSync('C:\\Users\\Administrator\\Desktop\\墨云(OpenClaw)\\工头派工台 正式版\\catherder.sqlite', { readOnly: true });
for (const r of db.prepare("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name").all()) {
  const cols = db.prepare(`SELECT name FROM pragma_table_info('${r.name}')`).all().map((c) => c.name);
  console.log(`${r.name}: ${cols.join(',')}`);
}
db.close();
