// 自查界面语言词典：
//  1) 代码里 tr('…') 用到的文案，词典里有没有（没有＝英文界面下不翻）
//  2) 静态页（index.html）里每段中文，过词典后还剩不剩中文（剩＝漏登记）
// 用法: node tools/i18n-check.cjs
const fs = require('fs');
const base = 'D:/06.AItools/openclaw/workspace/docs/项目/AI 代理 任务批发工具/app/frontend/dist/';
const unesc = (s) => s.replace(/\\'/g, "'").replace(/\\"/g, '"').replace(/\\\\/g, '\\');

const i18n = fs.readFileSync(base + 'i18n.js', 'utf8');
const dict = new Map();
for (const m of i18n.matchAll(/^\s*'((?:[^'\\]|\\.)*)'\s*:\s*'((?:[^'\\]|\\.)*)'/gm)) dict.set(unesc(m[1]), unesc(m[2]));
const sorted = [...dict.keys()].sort((a, b) => b.length - a.length);
const trSim = (s) => { let o = s; for (const zh of sorted) if (dict.has(zh) && o.includes(zh)) o = o.split(zh).join(dict.get(zh)); return o; };

const used = new Map();
for (const f of ['app.js', 'shot.js']) {
  const src = fs.readFileSync(base + f, 'utf8');
  for (const m of src.matchAll(/tr\(\s*'((?:[^'\\]|\\.)*)'/g)) {
    const s = unesc(m[1]);
    if (!used.has(s)) used.set(s, new Set());
    used.get(s).add(f);
  }
}
const miss = [...used.keys()].filter((k) => !dict.has(k));
console.log('词典条数=' + dict.size + '  tr()引用条数=' + used.size);
console.log('--- 用了但词典没有 ' + miss.length + ' 条 ---');
for (const k of miss) console.log('  ' + k.slice(0, 90) + '  [' + [...used.get(k)].join(',') + ']');

const html = fs.readFileSync(base + 'index.html', 'utf8').replace(/<!--[\s\S]*?-->/g, '');
// 跟 i18n.js 一样：带 data-no-i18n 的元素不参与翻译（语言下拉自己：「中文」不能被换成 Medium）。
const noI18n = /<([a-z]+)(?=[^>]*\bdata-no-i18n)[^>]*>[\s\S]*?<\/\1>/gi;
const htmlTr = html.replace(noI18n, '');
const segs = new Set();
for (const m of htmlTr.matchAll(/(title|placeholder|aria-label|alt)\s*=\s*"([^"]*)"/g)) if (/[一-鿿]/.test(m[2])) segs.add(m[2]);
for (const seg of htmlTr.split(/<[^>]+>/)) { const t = seg.replace(/\s+/g, ' ').trim(); if (t && /[一-鿿]/.test(t)) segs.add(t); }
const leftover = [...segs].map((s) => trSim(s)).filter((s) => /[一-鿿]/.test(s));
console.log('--- 静态页中文段 ' + segs.size + ' 段，英文界面下仍剩中文 ' + leftover.length + ' 段 ---');
for (const k of leftover.slice(0, 40)) console.log('  ' + k.slice(0, 95));
