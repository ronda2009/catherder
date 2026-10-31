// 查「那个运行到底收尾了没」：在几张相关表里找这个运行号，看有没有终态记录。
// 用法：node tools/peek-run-trace.cjs <run_id> [session_id]
const { DatabaseSync } = require('node:sqlite');
const f = (ms) => (!ms ? '(空)' : new Date(Number(ms)).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false }));
const db = new DatabaseSync('C:/Users/Administrator/.openclaw/agents/main/agent/openclaw-agent.sqlite', { readOnly: true });
const run = process.argv[2];
const sid = process.argv[3];
if (!run) { console.log('要给 run_id'); process.exit(1); }

const hits = (label, sql, args) => {
  try {
    const rows = db.prepare(sql).all(...args);
    console.log(`${label}: ${rows.length} 条`);
    for (const r of rows.slice(0, 4)) console.log('   ' + JSON.stringify(r).slice(0, 200));
  } catch (e) { console.log(`${label}: 查不了（${e.message}）`); }
};

hits('conversation_deliveries(run_id)', 'SELECT run_id,state,created_at FROM conversation_deliveries WHERE run_id=? ORDER BY created_at DESC LIMIT 4', [run]);
if (sid) {
  hits('transcript_events 里提到这个运行的事件', "SELECT seq,created_at,substr(event_json,1,120) j FROM transcript_events WHERE session_id=? AND event_json LIKE ? ORDER BY seq DESC LIMIT 4", [sid, '%' + run + '%']);
  hits('trajectory_runtime_events(run_id)', 'SELECT seq,created_at,substr(event_json,1,120) j FROM trajectory_runtime_events WHERE run_id=? ORDER BY seq DESC LIMIT 4', [run]);
}
hits('session_pending_inputs(run_id)', 'SELECT seq,state,accepted_at FROM session_pending_inputs WHERE run_id=? ORDER BY seq DESC LIMIT 4', [run]);
db.close();
