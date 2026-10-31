// 查 OpenClaw 会话库里某个窗口那一刻的状态与活动时间（查「误判闲置」用）。
// 用法：node tools\peek-window.cjs --key "agent:main:feishu:group:oc_c5e5…" --at "2026-10-26 21:08"
//      node tools\peek-window.cjs --statuses            → 看 status 都有哪些值、各几个
const { DatabaseSync } = require('node:sqlite');
const p = 'C:\\Users\\Administrator\\.openclaw\\agents\\main\\agent\\openclaw-agent.sqlite';
const a = process.argv.slice(2);
const arg = (k) => a[a.indexOf(k) + 1];
const db = new DatabaseSync(p, { readOnly: true });
const f = (ms) => (!ms ? '(空)' : new Date(Number(ms)).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false }));

if (a.includes('--statuses')) {
  for (const r of db.prepare(`SELECT status, COUNT(*) n FROM session_nodes GROUP BY status ORDER BY n DESC`).all()) {
    console.log('status=[' + r.status + '] ' + r.n + ' 个窗口');
  }
  const c = db.prepare(`SELECT COUNT(*) n FROM session_nodes WHERE archived_at IS NULL`).get();
  console.log('未归档窗口共 ' + c.n + ' 个');
  db.close();
  process.exit(0);
}

const key = arg('--key');
if (!key) { console.log('要给 --key（窗口键）或 --statuses'); process.exit(1); }
// 模型/服务商不在列里，在 entry_json 里（派工台 Go 那边也是这么读的）。
const row = db.prepare(`SELECT status, current_session_id, updated_at, created_at, last_activity_at, last_interaction_at,
  last_read_at, label, entry_json FROM session_nodes WHERE session_key=?`).get(key);
if (!row) { console.log('库里没有这个窗口键: ' + key); process.exit(1); }
console.log('窗口: ' + (row.label || key));
console.log('  status            = [' + row.status + ']');
console.log('  updated_at        = ' + f(row.updated_at));
console.log('  last_activity_at  = ' + f(row.last_activity_at));
console.log('  last_interaction  = ' + f(row.last_interaction_at));
let prov = '', model = '';
try {
  const e = JSON.parse(row.entry_json || '{}');
  prov = e.modelProvider || '';
  model = e.model || '';
} catch (_) { /* 读不到就算了 */ }
console.log('  model             = ' + (model || '(无)'));
console.log('  provider          = ' + (prov || '(无)'));

// 那一刻前后它自己有没有在产生事件（transcript_events 按 current_session_id 数）
const at = arg('--at');
if (at) {
  const t1 = new Date(at.replace(' ', 'T') + ':00+08:00').getTime();
  const sid = row.current_session_id;
  for (const span of [[t1 - 30 * 60000, t1], [t1, t1 + 30 * 60000]]) {
    const n = db.prepare(`SELECT COUNT(*) n, MIN(created_at) a, MAX(created_at) b FROM transcript_events
      WHERE session_id=? AND created_at>=? AND created_at<=?`).get(sid, span[0], span[1]);
    console.log('  该窗口事件 ' + f(span[0]) + ' ~ ' + f(span[1]) + ' → ' + n.n + ' 条' + (n.n ? '（最后一条 ' + f(n.b) + '）' : ''));
  }
}
db.close();
