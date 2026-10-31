#!/usr/bin/env node
// 读写测试版库里的任意设置键（命令行版 `set` 会把后面的参数全拼成值，带 -data 时尤其容易写脏）。
// 用法：
//   node tools/poke-setting.cjs <键>              只看
//   node tools/poke-setting.cjs <键> <值>         写入（值要空就用 ""）
//   node tools/poke-setting.cjs --all             列全部键
// 能验出什么：设置到底存成了什么（界面行为不对时先确认库里真是什么）。
// 自己的坑：只动测试版库（%APPDATA%\catherder）；正式版要用 peek-formal*.cjs 看，别拿这脚本改老板的库。
const path = require('node:path');
const { DatabaseSync } = require('node:sqlite');

const DB = path.join(process.env.APPDATA || '', 'catherder', 'catherder.sqlite');
const db = new DatabaseSync(DB);
const [k, v] = process.argv.slice(2);

if (k === '--all') {
  for (const r of db.prepare('SELECT key,value FROM settings ORDER BY key').all())
    console.log(`${r.key} = [${r.value}]  (len=${String(r.value).length})`);
} else if (v === undefined) {
  const r = db.prepare('SELECT value FROM settings WHERE key=?').get(k);
  console.log(`${k} = [${r ? r.value : '(未设)'}]  (len=${r ? String(r.value).length : 0})`);
} else {
  db.prepare(
    'INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value'
  ).run(k, v);
  const r = db.prepare('SELECT value FROM settings WHERE key=?').get(k);
  console.log(`写入后 ${k} = [${r.value}]  (len=${String(r.value).length})`);
}
db.close();
