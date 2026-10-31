#!/usr/bin/env node
// 读/写「自动关机」相关的设置与状态（测试版库专用，别拿去动正式版）。
// 用法：
//   node tools/poke-shutdown-state.cjs                     只看当前值
//   node tools/poke-shutdown-state.cjs --auto 1 --hours 03:00-03:05 --armed-ago 120
//     --auto 1|0            开/关自动关机
//     --hours HH:MM-HH:MM|empty   关机时段（empty＝不限）
//     --armed-ago N         把「准备关机的时刻」写成 N 秒前（负数＝未来 N 秒，用来模拟已武装还剩几秒；0 清零）
//     --gate-service 1|0    模型服务闸门（关掉它才好造「条件满足」）
//     --gate-gpu 1|0        显卡闸门
//     --include0 <窗口名>   把某个窗口取消勾选（不让它挡关机条件）
//     --include1 <窗口名>   重新勾选
//   node tools/poke-shutdown-state.cjs --restore           恢复本机默认（auto=0、时段空、标记 0、两道闸门开、工头派工台勾选）
// 能验出什么：程序启动后有没有把残留的关机准备作废、界面横幅该显示什么、取消按钮点了有没有立刻生效。
// 自己的坑：①只改库，不会替你把窗口变空闲 —— 本窗口正在干活就会挡条件，用 --include0 把它排除。
//          ②armed 只是「准备」，Windows 那条 shutdown 命令要到缓冲结束那一轮才下达；所以测完前把标记清掉就不会真关机。
//          ③真关机风险：条件一直满足 + 缓冲走完 = 机器真关。测前确认 --armed-ago 给足余量、测完 --restore。
const path = require('node:path');
const { DatabaseSync } = require('node:sqlite');

const DB = path.join(process.env.APPDATA || '', 'catherder', 'catherder.sqlite');
const db = new DatabaseSync(DB);
const argv = process.argv.slice(2);
const arg = (k) => {
  const i = argv.indexOf('--' + k);
  return i >= 0 ? argv[i + 1] : null;
};

const set = (k, v) =>
  db.prepare(
    'INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value'
  ).run(k, String(v));
const incl = (label, on) => db.prepare('UPDATE windows SET included=? WHERE label=?').run(on ? 1 : 0, label);

if (argv.includes('--restore')) {
  set('auto_shutdown', '0');
  set('shutdown_hours', '');
  set('shutdown_armed_at', '0');
  set('gate_service', '1');
  set('gate_gpu', '1');
  incl('工头派工台', true);
}
const a = arg('auto');
if (a !== null) set('auto_shutdown', a === '1' ? '1' : '0');
const h = arg('hours');
if (h !== null) set('shutdown_hours', h === 'empty' ? '' : h);
const ago = arg('armed-ago');
if (ago !== null) set('shutdown_armed_at', String(Date.now() - Number(ago) * 1000));
const gs = arg('gate-service');
if (gs !== null) set('gate_service', gs === '1' ? '1' : '0');
const gg = arg('gate-gpu');
if (gg !== null) set('gate_gpu', gg === '1' ? '1' : '0');
const i0 = arg('include0');
if (i0 !== null) incl(i0, false);
const i1 = arg('include1');
if (i1 !== null) incl(i1, true);

for (const k of ['auto_shutdown', 'shutdown_hours', 'shutdown_armed_at', 'gate_service', 'gate_gpu']) {
  const r = db.prepare('SELECT value FROM settings WHERE key=?').get(k);
  let v = r ? r.value : '';
  if (k === 'shutdown_armed_at' && Number(v) > 0)
    v += '（' + Math.round((Date.now() - Number(v)) / 1000) + ' 秒前，缓冲剩余 ' + Math.round((Number(v) + 60000 - Date.now()) / 1000) + ' 秒）';
  console.log(k + ' = [' + v + ']');
}
const w = db.prepare("SELECT included FROM windows WHERE label='工头派工台'").get();
console.log('工头派工台 勾选 = ' + (w ? (w.included ? '1' : '0') : '(库里没这个窗口)'));
db.close();
