// 台账列填充自查：列出「做了什么」等关键列为空的行号（我传错列名时会整列丢内容）。
// 用法：node tools/peek-ledger-cols.cjs --missing
// 背景：project-ledger-append.cjs 按固定列名填（版本/做了什么/状态/影响面/备份与回滚/验证/坑与禁忌/下一步），
//       传「阶段/内容/遗留/关联文件」这种自造键会被丢掉、那一列留空。
const fs = require('node:fs');
const path = require('node:path');
const CSV = path.join(__dirname, '..', '台账.csv');
const COLS = ['序号', '日期时间', '版本', '窗口', '做了什么', '状态', '影响面', '备份与回滚', '验证', '坑与禁忌', '下一步'];
const rows = fs.readFileSync(CSV, 'utf8').split(/\r?\n/).filter((x) => x.trim());
function parse(r) {
  const out = [];
  let cur = '';
  let q = false;
  for (let i = 0; i < r.length; i++) {
    const c = r[i];
    if (c === '"') {
      if (q && r[i + 1] === '"') { cur += '"'; i++; continue; }
      q = !q;
      continue;
    }
    if (c === ',' && !q) { out.push(cur); cur = ''; continue; }
    cur += c;
  }
  out.push(cur);
  return out;
}
if (process.argv[2] === '--missing') {
  const bad = [];
  rows.slice(1).forEach((r, i) => {
    const f = parse(r);
    const empty = [];
    [4, 8, 9, 10].forEach((c) => { if (!f[c] || !f[c].trim()) empty.push(COLS[c]); });
    if (empty.length) bad.push(`${f[0]}(第${i + 1}行): 空=${empty.join('/')}`);
  });
  console.log(`总行数=${rows.length - 1}，关键列有空的行=${bad.length}`);
  bad.forEach((b) => console.log('  ' + b));
} else {
  const n = Number(process.argv[2] || 2);
  console.log('表头: ' + rows[0]);
  for (const r of rows.slice(-n)) {
    const f = parse(r);
    console.log('--- 一行：字段数=' + f.length);
    f.forEach((v, i) => console.log(`  [${i}] ${COLS[i]} len=${v.length} | ${v.slice(0, 46).replace(/\n/g, ' ')}`));
  }
}
