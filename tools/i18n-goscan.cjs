// 扫 Go 侧「会到界面」的中文句子（feed 事件 / 弹窗问题 / 返回值 / 托盘），落 tmp/data/i18n-go.txt。
// 用法: node tools/i18n-goscan.cjs
const fs = require('fs');
const path = require('path');
const root = 'D:/06.AItools/openclaw/workspace/docs/项目/AI 代理 任务批发工具/app/internal/';
const CJK = /[一-鿿]/;
const out = [];
function walk(d) {
  for (const e of fs.readdirSync(d, { withFileTypes: true })) {
    const p = path.join(d, e.name);
    if (e.isDirectory()) { walk(p); continue; }
    if (!e.name.endsWith('.go') || e.name.endsWith('_test.go')) continue;
    const lines = fs.readFileSync(p, 'utf8').split(/\r?\n/);
    lines.forEach((ln, i) => {
      const t = ln.trim();
      if (/^\/\//.test(t)) return;
      // 字符串字面量里的中文
      for (const m of t.matchAll(/"((?:[^"\\]|\\.)*)"/g)) {
        const s = m[1].replace(/\\"/g, '"');
        if (!CJK.test(s)) continue;
        out.push({ f: e.name, line: i + 1, s });
      }
    });
  }
}
walk(root);
const seen = new Set();
const rows = out.filter((r) => { if (seen.has(r.s)) return false; seen.add(r.s); return true; });
fs.writeFileSync('D:/06.AItools/openclaw/workspace/tmp/data/i18n-go.txt',
  rows.map((r) => r.f + ' L' + r.line + '\t' + r.s).join('\n'), 'utf8');
console.log('Go 侧中文字符串（去重）' + rows.length + ' 条 → tmp/data/i18n-go.txt');
console.log('按文件：' + Object.entries(rows.reduce((m, r) => (m[r.f] = (m[r.f] || 0) + 1, m), {})).sort((a, b) => b[1] - a[1]).slice(0, 12).map(([f, n]) => f + '=' + n).join('  '));
