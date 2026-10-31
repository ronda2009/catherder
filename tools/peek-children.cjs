// 查「某个窗口有没有挂着的子任务还在跑」——派工台判闲置时只看窗口自己那一回合，看不见它派出去的子任务。
// 用法：node tools\peek-children.cjs --label 绘图员 --at "2026-10-26 21:08"
//      node tools\peek-children.cjs --label 绘图员            → 列这个窗口名下所有子会话
const { DatabaseSync } = require('node:sqlite');
const p = 'C:\\Users\\Administrator\\.openclaw\\agents\\main\\agent\\openclaw-agent.sqlite';
const a = process.argv.slice(2);
const arg = (k) => a[a.indexOf(k) + 1];
const label = arg('--label'), key = arg('--key'), at = arg('--at');
if (!label && !key) { console.log('要给 --label 或 --key'); process.exit(1); }
const db = new DatabaseSync(p, { readOnly: true });
const f = (ms) => (!ms ? '(空)' : new Date(Number(ms)).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false }));
const t = at ? new Date(at.replace(' ', 'T') + ':00+08:00').getTime() : null;

const NAME = `COALESCE(NULLIF(n.label,''), json_extract(n.entry_json,'$.label'), json_extract(n.entry_json,'$.displayName'), n.session_key)`;
const parents = label
  ? db.prepare(`SELECT session_key k FROM session_nodes n WHERE ${NAME} LIKE ?`).all('%' + label + '%')
  : [{ k: key }];
if (!parents.length) { console.log('找不到窗口'); process.exit(1); }

for (const par of parents) {
  console.log('=== 父窗口 ' + par.k);
  // 1) session_nodes 里以它为父的会话（子代理/后台任务）
  const kids = db.prepare(`SELECT n.session_key k, n.status, n.created_at, n.updated_at, n.last_activity_at,
    json_extract(n.entry_json,'$.model') model FROM session_nodes n
    WHERE n.parent_session_key=? ORDER BY n.created_at DESC LIMIT 40`).all(par.k);
  console.log('  session_nodes 里以它为父的会话：' + kids.length + ' 个');
  for (const r of kids) {
    const live = t ? (r.created_at <= t && (!r.updated_at || r.updated_at >= t)) : true;
    console.log('   ' + (live ? '●' : ' ') + ' ' + r.k + ' status=[' + r.status + '] 建于 ' + f(r.created_at) +
      ' 最后动 ' + f(r.last_activity_at || r.updated_at) + ' 模型=' + (r.model || '?'));
  }
  // 2) session_windows 里以它为父的回合（带起止，能判「那一刻在不在跑」）
  const runs = db.prepare(`SELECT session_id, session_key, status, started_at, ended_at, model,
    parent_session_key FROM session_windows WHERE parent_session_key=? ORDER BY COALESCE(started_at, created_at) DESC LIMIT 40`).all(par.k);
  console.log('  session_windows 里以它为父的回合：' + runs.length + ' 个');
  for (const r of runs) {
    const s = Number(r.started_at || 0), e = Number(r.ended_at || 0);
    const live = t ? (s && s <= t && (e === 0 || e >= t)) : true;
    console.log('   ' + (live ? '●' : ' ') + ' ' + String(r.session_id).slice(0, 13) + ' [' + r.status + '] ' +
      f(r.started_at) + ' → ' + f(r.ended_at) + ' 模型=' + (r.model || '?'));
  }
}
db.close();
