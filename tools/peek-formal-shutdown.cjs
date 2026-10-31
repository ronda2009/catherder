#!/usr/bin/env node
// 直接读正式版库里的「自动关机」三项（只读，不改）。
// 用法：node tools/peek-formal-shutdown.cjs
// 能验出什么：正式版有没有挂着「准备关机」、老板的开关与时段设成什么了。
const { DatabaseSync } = require('node:sqlite');
const DB = 'C:\\Users\\Administrator\\Desktop\\墨云(OpenClaw)\\工头派工台 正式版\\catherder.sqlite';
const db = new DatabaseSync(DB, { readOnly: true });
for (const k of ['auto_shutdown', 'shutdown_hours', 'shutdown_armed_at', 'gate_service', 'gate_gpu']) {
  const r = db.prepare('SELECT value FROM settings WHERE key=?').get(k);
  let v = r ? r.value : '(未设)';
  if (k === 'shutdown_armed_at' && v !== '(未设)' && Number(v) > 0)
    v += '（' + Math.round((Date.now() - Number(v)) / 1000) + ' 秒前）';
  console.log(k + ' = [' + v + ']');
}
db.close();
