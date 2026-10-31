// 查「那一刻机器上到底有哪些会话回合在跑」——派工台判闲置只看目标窗口自己那一回合，
// 看不见它自己挂出去的任务（子会话/别的窗口/外部进程）。这个脚本按时间倒查，用来对质。
// 用法：node tools\peek-activity.cjs --at "2026-10-26 21:08" [--near 15]
//      --near 分钟：也算进「那一刻前后 N 分钟内还在产生事件」的会话
const { DatabaseSync } = require('node:sqlite');
const p = 'C:\\Users\\Administrator\\.openclaw\\agents\\main\\agent\\openclaw-agent.sqlite';
const a = process.argv.slice(2);
const arg = (k) => a[a.indexOf(k) + 1];
const at = arg('--at');
if (!at) { console.log('要给 --at（例 "2026-10-26 21:08"）'); process.exit(1); }
const near = Number(arg('--near') || 15) * 60000;
const db = new DatabaseSync(p, { readOnly: true });
const f = (ms) => (!ms ? '(空)' : new Date(Number(ms)).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false }));
const t = new Date(at.replace(' ', 'T') + ':00+08:00').getTime();

const NAME = `COALESCE(NULLIF(n.label,''), json_extract(n.entry_json,'$.label'), json_extract(n.entry_json,'$.displayName'))`;

console.log('=== 那一刻「起止跨过它」的回合（session_windows）');
const live = db.prepare(`SELECT w.session_id, w.session_key, w.status, w.started_at, w.ended_at, w.model, w.parent_session_key,
  COALESCE(NULLIF(w.display_name,''), ${NAME}, w.session_key) lbl
  FROM session_windows w LEFT JOIN session_nodes n ON n.session_key=w.session_key
  WHERE COALESCE(w.started_at,w.created_at)<=? AND (COALESCE(w.ended_at,0)=0 OR w.ended_at>=?)
  ORDER BY COALESCE(w.started_at,w.created_at) DESC LIMIT 30`).all(t, t);
console.log('共 ' + live.length + ' 个');
for (const r of live) {
  console.log('  ' + String(r.session_id).slice(0, 13) + ' [' + r.status + '] ' + f(r.started_at) + ' → ' + f(r.ended_at) +
    ' 模型=' + (r.model || '?') + ' 窗口=' + (r.lbl || r.session_key) + (r.parent_session_key ? ' 父=' + r.parent_session_key : ''));
}

console.log('=== 那一刻前后 ' + (near / 60000) + ' 分钟内还在产生事件的会话');
const act = db.prepare(`SELECT e.session_id, MIN(e.created_at) a, MAX(e.created_at) b, COUNT(*) n,
  COALESCE(NULLIF(w.session_key,''), '?') k, COALESCE(NULLIF(w.display_name,''), ${NAME}, '') lbl
  FROM transcript_events e
  LEFT JOIN session_windows w ON w.session_id=e.session_id
  LEFT JOIN session_nodes n ON n.session_key=w.session_key
  WHERE e.created_at>=? AND e.created_at<=?
  GROUP BY e.session_id HAVING n>20 ORDER BY b DESC LIMIT 20`).all(t - near, t + near);
console.log('共 ' + act.length + ' 个');
for (const r of act) {
  console.log('  ' + String(r.session_id).slice(0, 13) + ' 事件 ' + r.n + ' 条 ' + f(r.a) + ' → ' + f(r.b) + ' 窗口=' + (r.lbl || r.k || '?'));
}
db.close();
