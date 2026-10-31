// 按名字在 OpenClaw 会话库里找窗口/回合，并列出它昨晚那一刻前后的活动（查「谁在占着模型」）。
// 用法：node tools\peek-name.cjs 小龙虾 [--at "2026-10-26 21:08"]
const { DatabaseSync } = require('node:sqlite');
const p = 'C:\\Users\\Administrator\\.openclaw\\agents\\main\\agent\\openclaw-agent.sqlite';
const a = process.argv.slice(2);
const arg = (k) => a[a.indexOf(k) + 1];
const want = a[0];
if (!want) { console.log('要给名字（例：小龙虾）'); process.exit(1); }
const at = arg('--at');
const db = new DatabaseSync(p, { readOnly: true });
const f = (ms) => (!ms ? '(空)' : new Date(Number(ms)).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false }));
const t = at ? new Date(at.replace(' ', 'T') + ':00+08:00').getTime() : null;

const NAME = `COALESCE(NULLIF(n.label,''), json_extract(n.entry_json,'$.label'), json_extract(n.entry_json,'$.displayName'), n.session_key)`;
const hits = db.prepare(`SELECT n.session_key k, n.status, n.created_at, n.updated_at, n.last_activity_at,
  json_extract(n.entry_json,'$.model') model, json_extract(n.entry_json,'$.modelProvider') prov,
  ${NAME} lbl FROM session_nodes n WHERE ${NAME} LIKE ? ORDER BY n.created_at DESC LIMIT 20`).all('%' + want + '%');
console.log('名字含「' + want + '」的会话：' + hits.length + ' 个');
for (const r of hits) {
  console.log('  ' + r.lbl + '\n     key=' + r.k + '\n     status=[' + r.status + '] 建于 ' + f(r.created_at) +
    ' 最后活动 ' + f(r.last_activity_at || r.updated_at) + ' 模型=' + (r.model || '?') + '/' + (r.prov || '?'));
}

if (t) {
  console.log('=== 这些会话名下、那一刻前后 3 小时内的回合');
  for (const h of hits) {
    const runs = db.prepare(`SELECT session_id, status, started_at, ended_at, model FROM session_windows
      WHERE session_key=? ORDER BY COALESCE(started_at, created_at) DESC LIMIT 12`).all(h.k);
    for (const r of runs) {
      const s = Number(r.started_at || 0), e = Number(r.ended_at || 0);
      if (!s || Math.abs(s - t) > 3 * 3600000) continue;
      const live = (s <= t) && (e === 0 || e >= t);
      console.log('  ' + (live ? '●跨着那一刻' : ' ') + ' ' + h.lbl + ' 回合 ' + String(r.session_id).slice(0, 13) +
        ' [' + r.status + '] ' + f(r.started_at) + ' → ' + f(r.ended_at) + ' 模型=' + (r.model || '?'));
    }
  }
}
db.close();
