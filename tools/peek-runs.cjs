// 查某个窗口在一段时间里到底有没有在跑（按「回合」看，不按当前 session_id）。
// 关键：transcript_events 按 session_id 存，而 session_nodes.current_session_id 每跑一回合就换，
// 只拿 current_session_id 查历史会漏掉之前那些回合（探测手段自己就会给假结论）。
// 回合历史走 session_windows（一行一个回合，带 started_at/ended_at/status）+ transcript_events 事件数。
// 用法：node tools\peek-runs.cjs --label 绘图员 --from "2026-10-26 20:00" --to "2026-10-26 22:00"
//      node tools\peek-runs.cjs --key "agent:main:feishu:group:oc_c5e5…" --from ... --to ...
//      加 --all 看这个窗口名下全部回合（不卡时间，用来确认表里到底有没有这个窗口的数据）
const { DatabaseSync } = require('node:sqlite');
const p = 'C:\\Users\\Administrator\\.openclaw\\agents\\main\\agent\\openclaw-agent.sqlite';
const a = process.argv.slice(2);
const arg = (k) => a[a.indexOf(k) + 1];
const key = arg('--key'), label = arg('--label');
const from = arg('--from'), to = arg('--to');
const all = a.includes('--all');
if (!key && !label) { console.log('要给 --key 或 --label'); process.exit(1); }
const db = new DatabaseSync(p, { readOnly: true });
const f = (ms) => (!ms ? '(空)' : new Date(Number(ms)).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false }));
const t0 = from ? new Date(from.replace(' ', 'T') + ':00+08:00').getTime() : 0;
const t1 = to ? new Date(to.replace(' ', 'T') + ':00+08:00').getTime() : 9e15;

// 名字优先从 entry_json 取（实测 session_nodes.label 常为空）。
const SEL = `SELECT session_key k, COALESCE(NULLIF(label,''), json_extract(entry_json,'$.label'),
  json_extract(entry_json,'$.displayName')) l, status, current_session_id cur FROM session_nodes`;
const win = label
  ? db.prepare(SEL + ` WHERE COALESCE(NULLIF(label,''), json_extract(entry_json,'$.label'), json_extract(entry_json,'$.displayName'), session_key) LIKE ?`).all('%' + label + '%')
  : [db.prepare(SEL + ` WHERE session_key=?`).get(key)];
if (!win.length || !win[0]) { console.log('库里找不到窗口（label 与 session_key 都没对上）'); process.exit(1); }

for (const w of win) {
  console.log('=== 窗口 ' + (w.l || w.k));
  console.log('    现在的 status = [' + w.status + ']  现在的回合 id = ' + w.cur);
  const total = db.prepare(`SELECT COUNT(*) n FROM session_windows WHERE session_key=?`).get(w.k);
  console.log('    session_windows 里这个窗口名下共 ' + total.n + ' 个回合');
  const runs = db.prepare(`SELECT session_id, status, started_at, ended_at, created_at, transcript_updated_at, model,
    (SELECT COUNT(*) FROM transcript_events e WHERE e.session_id = w.session_id) n
    FROM session_windows w WHERE session_key=?
    ORDER BY COALESCE(started_at, created_at) DESC LIMIT 60`).all(w.k);
  let shown = 0;
  for (const r of runs) {
    const anchor = Number(r.started_at || r.created_at || 0);
    if (!all && (anchor < t0 || anchor > t1)) continue;
    shown++;
    console.log('    回合 ' + String(r.session_id).slice(0, 13) + ' status=[' + r.status + '] 起 ' + f(r.started_at) +
      ' 止 ' + f(r.ended_at) + ' 事件 ' + r.n + ' 条 模型=' + (r.model || '?'));
  }
  if (!shown) console.log('    （这段时间内没有回合记录' + (all ? '' : '；加 --all 看全部') + '）');
  const pend = db.prepare(`SELECT seq, state, accepted_at, consumed_event_id FROM session_pending_inputs
    WHERE session_key=? AND accepted_at>=? AND accepted_at<=? ORDER BY seq`).all(w.k, t0 - 6 * 3600000, t1);
  for (const r of pend) console.log('    待处理输入 seq=' + r.seq + ' state=[' + r.state + '] 收下于 ' + f(r.accepted_at) + ' 消费事件=' + (r.consumed_event_id ? '有' : '无'));
}
db.close();
