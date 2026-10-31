// 看某个会话最近的对话事件原文（前若干字），判「它到底在干什么、有没有收尾」。
// 用法：node tools/peek-session-events.cjs <session_id> [条数]
// 事件正文存在 event_json；老事件压成 event_zstd（要 zstd 解，本机没库，这里只报「压缩 N B」）。
const { DatabaseSync } = require('node:sqlite');
const f = (ms) => (!ms ? '(空)' : new Date(Number(ms)).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false }));
const db = new DatabaseSync('C:/Users/Administrator/.openclaw/agents/main/agent/openclaw-agent.sqlite', { readOnly: true });
const sid = process.argv[2];
const n = Number(process.argv[3]) || 6;
if (!sid) { console.log('要给 session_id'); process.exit(1); }
const rows = db.prepare('SELECT seq,created_at,event_json,event_zstd FROM transcript_events WHERE session_id=? ORDER BY seq DESC LIMIT ?').all(sid, n).reverse();
console.log(`会话 ${sid} 最近 ${rows.length} 条事件（正序）：`);
for (const r of rows) {
  let body = r.event_json;
  if (!body && r.event_zstd) body = `(压缩 ${r.event_zstd.length}B)`;
  const t = String(body || '').replace(/\s+/g, ' ');
  let type = '';
  try { type = JSON.parse(r.event_json || '{}').type || ''; } catch (_) { type = '?'; }
  console.log(`  #${r.seq} ${f(r.created_at)} [${type}] ${t.slice(0, 150)}`);
}
db.close();
