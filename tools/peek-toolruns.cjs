// 查某个窗口那一刻有没有「工具/后台任务还在跑」的记录（message_tool_run_outcomes 带 run_status）。
// 派工台判闲置只看窗口那一回合结束没结束，看不见它挂出去的后台活——这个脚本就是去查有没有这种活。
// 用法：node tools\peek-toolruns.cjs --label 绘图员 --from "2026-10-26 20:30" --to "2026-10-26 21:30"
const { DatabaseSync } = require('node:sqlite');
const p = 'C:\\Users\\Administrator\\.openclaw\\agents\\main\\agent\\openclaw-agent.sqlite';
const a = process.argv.slice(2);
const arg = (k) => a[a.indexOf(k) + 1];
const label = arg('--label'), key = arg('--key');
const from = arg('--from'), to = arg('--to');
if ((!label && !key) || !from || !to) { console.log('要给 --label/--key 与 --from/--to'); process.exit(1); }
const db = new DatabaseSync(p, { readOnly: true });
const f = (ms) => (!ms ? '(空)' : new Date(Number(ms)).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false }));
const t0 = new Date(from.replace(' ', 'T') + ':00+08:00').getTime();
const t1 = new Date(to.replace(' ', 'T') + ':00+08:00').getTime();

const NAME = `COALESCE(NULLIF(n.label,''), json_extract(n.entry_json,'$.label'), json_extract(n.entry_json,'$.displayName'), n.session_key)`;
const keys = label
  ? db.prepare(`SELECT session_key k, ${NAME} l FROM session_nodes n WHERE ${NAME} LIKE ?`).all('%' + label + '%')
  : [{ k: key, l: key }];
if (!keys.length) { console.log('找不到窗口'); process.exit(1); }

for (const w of keys) {
  console.log('=== 窗口 ' + (w.l || w.k));
  const runs = db.prepare(`SELECT run_id, provider, model, outcome, run_status, occurred_at
    FROM message_tool_run_outcomes WHERE session_key=? AND occurred_at>=? AND occurred_at<=?
    ORDER BY occurred_at DESC LIMIT 25`).all(w.k, t0, t1);
  console.log('  这段时间的工具回合记录：' + runs.length + ' 条');
  for (const r of runs) {
    console.log('   ' + f(r.occurred_at) + ' run_status=[' + r.run_status + '] outcome=[' + r.outcome + '] 模型=' + (r.model || '?') + '/' + (r.provider || '?'));
  }
  // 那一刻窗口自己那一回合的状态（session_nodes.status 为 NULL 才算在跑）
  const n = db.prepare(`SELECT status, updated_at, last_activity_at FROM session_nodes WHERE session_key=?`).get(w.k);
  if (n) console.log('  窗口现在的 status=[' + n.status + '] 最后活动 ' + f(n.last_activity_at || n.updated_at));
}
db.close();
