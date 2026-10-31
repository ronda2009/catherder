// 打印台账里指定序号那一行的全部列原文（照格式补行时用）。
// 用法：node tools/peek-ledger-row.cjs <序号>
const fs = require('node:fs');
const path = require('node:path');
const CSV = path.join(__dirname, '..', '台账.csv');
const NAMES = ['序号', '日期时间', '版本', '窗口', '做了什么', '状态', '影响面', '备份与回滚', '验证', '坑与禁忌', '下一步'];
const want = process.argv[2];
const rows = fs.readFileSync(CSV, 'utf8').split(/\r?\n/).filter((x) => x.trim());
function parse(r) {
  const o = [];
  let c = '';
  let q = false;
  for (let i = 0; i < r.length; i++) {
    const ch = r[i];
    if (ch === '"') {
      if (q && r[i + 1] === '"') { c += '"'; i++; continue; }
      q = !q;
      continue;
    }
    if (ch === ',' && !q) { o.push(c); c = ''; continue; }
    c += ch;
  }
  o.push(c);
  return o;
}
for (const r of rows.slice(1)) {
  const f = parse(r);
  if (f[0] === want) {
    f.forEach((v, i) => console.log(`[${i}] ${NAMES[i]} (len=${v.length}):\n${v}\n`));
    process.exit(0);
  }
}
console.log('没找到序号 ' + want);
