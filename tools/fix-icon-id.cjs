// 把 .syso 里的组图标资源编号改成指定值（默认 3）。
// 为什么：Windows 外壳与 Wails 都按编号取图标，Wails 写死只认编号 3（winc.AppIconID=3）。
// 我们的 .ico 编进去后组图标编号是 1，Wails 取不到 → 任务栏/标题栏显示通用图标。
// 用法: node tools/fix-icon-id.cjs <文件.syso> [新编号=3]
const fs = require('fs');

const file = process.argv[2];
const want = Number(process.argv[3] || 3);
if (!file) {
  console.log('用法: node tools/fix-icon-id.cjs <文件.syso> [新编号=3]');
  process.exit(1);
}
const buf = fs.readFileSync(file);

// 定位 .rsrc（COFF：20 字节文件头后接节表）
const nSec = buf.readUInt16LE(2);
let sec = 20, rsrc = -1;
for (let i = 0; i < nSec; i++) {
  const name = buf.toString('latin1', sec, sec + 8).replace(/\0/g, '');
  if (name === '.rsrc') rsrc = buf.readUInt32LE(sec + 20);
  sec += 40;
}
if (rsrc < 0) {
  console.log('这个文件里没有 .rsrc 节');
  process.exit(1);
}

// 顶层资源目录：类型表
const named = buf.readUInt16LE(rsrc + 12);
const ids = buf.readUInt16LE(rsrc + 14);
let p = rsrc + 16 + named * 8;
const RT_GROUP_ICON = 14;
let hit = -1;
for (let i = 0; i < ids; i++, p += 8) {
  if (buf.readUInt32LE(p) === RT_GROUP_ICON) {
    hit = buf.readUInt32LE(p + 4) & 0x7fffffff;
    break;
  }
}
if (hit < 0) {
  console.log('没有 RT_GROUP_ICON(14) 类型');
  process.exit(1);
}

// 该类型下的目录：条目表（无名条目按编号升序）
const dir = rsrc + hit;
const dNamed = buf.readUInt16LE(dir + 12);
const dIds = buf.readUInt16LE(dir + 14);
const e0 = dir + 16 + dNamed * 8;
if (dIds < 1) {
  console.log('组图标目录是空的');
  process.exit(1);
}
const old = buf.readUInt32LE(e0);
buf.writeUInt32LE(want, e0);
fs.writeFileSync(file, buf);
console.log(`组图标编号 ${old} → ${want}（条目 ${dIds} 个，文件 ${buf.length} B）`);
