// 台账里「谁写的、哪列空」一览（我传错列名时会整列丢内容，先分清是不是我造成的）。
// 用法：node tools/peek-ledger-who.cjs
const fs = require('node:fs');
const path = require('node:path');
const CSV = path.join(__dirname, '..', '台账.csv');
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
const NAMES = ['序号', '日期时间', '版本', '窗口', '做了什么', '状态', '影响面', '备份与回滚', '验证', '坑与禁忌', '下一步'];
const byWindow = {};
rows.slice(1).forEach((r) => {
  const f = parse(r);
  const empties = [4, 8, 9, 10].filter((c) => !(f[c] || '').trim()).map((c) => NAMES[c]);
  const w = f[3] || '(空窗口)';
  byWindow[w] = byWindow[w] || { n: 0, bad: 0, badIds: [] };
  byWindow[w].n++;
  if (empties.length) {
    byWindow[w].bad++;
    byWindow[w].badIds.push(f[0] + ':' + empties.join('/'));
  }
});
for (const [w, v] of Object.entries(byWindow)) {
  console.log(`窗口=${w} 行数=${v.n} 有空列=${v.bad}`);
  if (v.bad) console.log('   ' + v.badIds.join('  '));
}
