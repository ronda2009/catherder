// 补回台账里被我写丢的列：我传错了列名（用了「阶段/内容/遗留/关联文件」这些自造键），
// project-ledger-append.cjs 只认固定列名，于是「做了什么」整列空、正文整段丢在 CSV 外。
// 正文没真丢——源 json 还在 tmp\data\ledger_taskwholesale_rNN.json，按「验证」列原文精确匹配找回。
// 用法：node tools/fix-ledger-cols.cjs           只报会怎么改（不动文件）
//      node tools/fix-ledger-cols.cjs --apply    写回（先自己备份）
// 自己的坑：①匹配靠「验证」列全文相等，匹配不上就跳过、绝不猜；②序号 20-27 那 8 行整行全空，
//          不是我这几轮写的，本脚本不碰；③改前必须先备份 台账.csv。
const fs = require('node:fs');
const path = require('node:path');
const CSV = path.join(__dirname, '..', '台账.csv');
const SRC_DIR = path.join(__dirname, '..', '..', '..', '..', 'tmp', 'data');
const COLS = ['序号', '日期时间', '版本', '窗口', '做了什么', '状态', '影响面', '备份与回滚', '验证', '坑与禁忌', '下一步'];
const APPLY = process.argv.includes('--apply');

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
const esc = (s) => {
  let t = String(s).replace(/\r\n?|\n/g, '；').replace(/\s+/g, ' ').trim();
  if (/[",;]/.test(t) || /^\s/.test(String(s))) t = '"' + t.replace(/"/g, '""') + '"';
  return t;
};

const raw = fs.readFileSync(CSV, 'utf8');
const eol = raw.includes('\r\n') ? '\r\n' : '\n';
const lines = raw.split(/\r?\n/);
const header = lines[0];
const body = lines.slice(1).filter((x) => x.trim());

// 源文件按 mtime 排序（台账序号是按时间递增的）
const srcs = fs.readdirSync(SRC_DIR)
  .filter((f) => /^ledger_taskwholesale_r\d+\.json$/.test(f))
  .map((f) => {
    const p = path.join(SRC_DIR, f);
    let j;
    try { j = JSON.parse(fs.readFileSync(p, 'utf8')); } catch (e) { return null; }
    if (!Array.isArray(j) || !j[0] || !j[0]['验证']) return null;
    return { file: f, mtime: fs.statSync(p).mtime, row: j[0] };
  })
  .filter(Boolean)
  .sort((a, b) => a.mtime - b.mtime);

const out = [];
const report = [];
let used = new Set();
for (const line of body) {
  const f = parse(line);
  if (f[4] && f[4].trim()) { out.push(line); continue; }   // 「做了什么」有内容 → 不动
  if (!f[8] || !f[8].trim()) { out.push(line); report.push(`${f[0]}: 「验证」也空（整行空），本脚本不碰`); continue; }
  const hit = srcs.find((s) => !used.has(s.file) && s.row['验证'].trim() === f[8].trim());
  if (!hit) { out.push(line); report.push(`${f[0]}: 找不到对应源文件，跳过（不猜）`); continue; }
  used.add(hit.file);
  const r = hit.row;
  const did = `【${r['阶段']}】\n${r['内容']}\n关联文件：${r['关联文件'] || ''}`;
  const next = r['遗留'] || '';
  const dt = hit.mtime.toLocaleString('zh-CN', { hour12: false }).replace(/\//g, '-');
  const nf = [f[0], dt, f[2], f[3], did, '已完成', f[6], f[7], f[8], '', next];
  out.push(nf.map(esc).join(','));
  report.push(`${f[0]}:  ← ${hit.file}  做了什么 ${f[4].length}→${did.length} 字，补 日期时间=${dt}、状态=已完成、下一步 ${f[10].length}→${next.length} 字`);
}

report.forEach((x) => console.log(x));
console.log(`共处理 ${report.length} 行；${APPLY ? '已写回' : '未写回（加 --apply 才写）'}`);
if (APPLY) {
  fs.writeFileSync(CSV, [header, ...out, ''].join(eol), 'utf8');
  const chk = fs.readFileSync(CSV, 'utf8').split(/\r?\n/).filter((x) => x.trim());
  console.log(`写回后行数=${chk.length - 1}（应等于 ${body.length}）`);
  if (chk.length - 1 !== body.length) {
    console.log('❌ 行数不对，已用备份恢复');
    fs.writeFileSync(CSV, raw, 'utf8');
  }
}
