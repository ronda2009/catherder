// 列出前端「写界面文字」的位置，并标出哪些带变量（带变量的必须改代码，纯静态的可整体换）。
// 用法: node tools/i18n-writesites.cjs
const fs = require('fs');
const base = 'D:/06.AItools/openclaw/workspace/docs/项目/AI 代理 任务批发工具/app/frontend/dist/';
const CJK = /[\u4e00-\u9fff]/;
const WRITE = /\.textContent\s*=|\.innerHTML\s*=|\.title\s*=|\.placeholder\s*=|setAttribute\(/;
const rows = [];
for (const f of ['app.js', 'shot.js']) {
  const lines = fs.readFileSync(base + f, 'utf8').split(/\r?\n/);
  lines.forEach((ln, i) => {
    if (!WRITE.test(ln) || !CJK.test(ln)) return;
    const t = ln.trim();
    const dyn = /\$\{/.test(t) || /['"`]\s*\+|\+\s*['"`]/.test(t);
    rows.push({ f, line: i + 1, dyn, t });
  });
}
const dyn = rows.filter((r) => r.dyn);
fs.writeFileSync('D:/06.AItools/openclaw/workspace/tmp/data/i18n-writesites.json', JSON.stringify(rows, null, 1));
console.log('写入位置总数=' + rows.length + '  带变量=' + dyn.length + '  纯静态=' + (rows.length - dyn.length));
console.log('--- 带变量的（要改代码） ---');
for (const r of dyn) console.log(r.f + ' L' + r.line + ': ' + r.t.slice(0, 110));
