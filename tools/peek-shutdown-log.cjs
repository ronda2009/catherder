#!/usr/bin/env node
// 查关机相关的历史事件（正式版/测试版都能查），时间按本地时区格式化打印。
// 用法：node tools/peek-shutdown-log.cjs [关键字]   默认关键字「关机」
// 能验出什么：谁在什么时候下达了关机、有没有被撤销、程序是什么时候启动的。
const path = require('node:path');
const fs = require('node:fs');
const { DatabaseSync } = require('node:sqlite');

const FORMAL = 'C:\\Users\\Administrator\\Desktop\\墨云(OpenClaw)\\工头派工台 正式版\\catherder.sqlite';
const TEST = path.join(process.env.APPDATA || '', 'catherder', 'catherder.sqlite');
const kw = process.argv[2] || '关机';

function fmt(ms) {
  const d = new Date(ms);
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

for (const db of [FORMAL, TEST]) {
  if (!fs.existsSync(db)) continue;
  const which = db === FORMAL ? '正式版' : '测试版';
  let handle;
  try {
    handle = new DatabaseSync(db, { readOnly: true });
    const rows = handle
      .prepare("SELECT at, kind, detail FROM events WHERE detail LIKE ? OR kind='shutdown' ORDER BY at DESC LIMIT 20")
      .all('%' + kw + '%');
    console.log(`【${which}】命中 ${rows.length} 条（关键字「${kw}」）`);
    for (const r of rows) console.log(`  ${fmt(r.at)} [${r.kind}] ${String(r.detail).slice(0, 110)}`);
  } catch (e) {
    console.log(`【${which}】查不了：${e.message}`);
  } finally {
    if (handle) handle.close();
  }
}
