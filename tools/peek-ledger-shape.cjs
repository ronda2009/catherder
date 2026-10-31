// 台账结构终检：每行字段数是否等于表头列数、序号是否连续。
// 用法：node tools/peek-ledger-shape.cjs
const fs = require('node:fs');
const path = require('node:path');
const rows = fs.readFileSync(path.join(__dirname, '..', '台账.csv'), 'utf8').split(/\r?\n/).filter((x) => x.trim());
function parse(r) {
  const o = [];
  let c = '', q = false;
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
const ncol = parse(rows[0]).length;
const bad = [];
rows.slice(1).forEach((r, i) => {
  const n = parse(r).length;
  if (n !== ncol) bad.push(`${i + 1}:字段数=${n}`);
});
const seqs = rows.slice(1).map((r) => Number(parse(r)[0]));
const gaps = [];
seqs.forEach((s, i) => { if (s !== i + 1) gaps.push(`第${i + 1}行序号=${s}`); });
console.log(`列数=${ncol} 数据行=${rows.length - 1} 字段数不对的行=${bad.length}${bad.length ? ' → ' + bad.slice(0, 5).join(' ') : ''}`);
console.log(`序号连续=${gaps.length === 0}${gaps.length ? ' → ' + gaps.slice(0, 5).join(' ') : ''} 末序号=${seqs[seqs.length - 1]}`);
