// 列出 app.js / shot.js 里含中文的行，并分类：界面可见 / 只进日志(debug) / 注释。
// 用法: node tools/i18n-classify.cjs   → 落 tmp/data/i18n-lines.txt，只打各类条数
const fs = require('fs');
const base = 'D:/06.AItools/openclaw/workspace/docs/项目/AI 代理 任务批发工具/app/frontend/dist/';
const CJK = /[\u4e00-\u9fff]/;
const DEBUG = /Debug\(|say\(|console\./;
const UI = /\.textContent\s*=|\.innerHTML\s*=|\.title\s*=|\.placeholder\s*=|setAttribute\(|feed\(|alert\(|confirm\(|<option|textContent =|innerHTML =/;
const out = [];
for (const f of ['app.js', 'shot.js']) {
  const lines = fs.readFileSync(base + f, 'utf8').split(/\r?\n/);
  lines.forEach((ln, i) => {
    if (!CJK.test(ln)) return;
    const t = ln.trim();
    let cls;
    if (/^\/\/|^\*|^\/\*/.test(t)) cls = '注释';
    else if (DEBUG.test(t)) cls = '日志';
    else if (UI.test(t)) cls = '界面';
    else cls = '待定';
    out.push({ f, line: i + 1, cls, t });
  });
}
const by = {};
for (const r of out) (by[r.cls] = by[r.cls] || []).push(r);
fs.writeFileSync('D:/06.AItools/openclaw/workspace/tmp/data/i18n-lines.txt',
  out.map((r) => r.cls + '\t' + r.f + ' L' + r.line + '\t' + r.t).join('\n'), 'utf8');
for (const k of Object.keys(by)) console.log(k + ' = ' + by[k].length + ' 行');
console.log('合计 ' + out.length + ' 行 → tmp/data/i18n-lines.txt');
