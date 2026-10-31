// 把词典里所有「长英文句」导出来人工审（短词条/片段没法机器判语法）。
// 用法：node tools\i18n-review.cjs [最少长度]   默认 70 字符以上
const fs = require('node:fs');
const base = 'D:/06.AItools/openclaw/workspace/docs/项目/AI 代理 任务批发工具/app/frontend/dist/';
const unesc = (s) => s.replace(/\\'/g, "'").replace(/\\"/g, '"').replace(/\\\\/g, '\\');
const min = Number(process.argv[2]) || 70;
const src = fs.readFileSync(base + 'i18n.js', 'utf8');
const out = [];
for (const m of src.matchAll(/^\s*'((?:[^'\\]|\\.)*)'\s*:\s*'((?:[^'\\]|\\.)*)'/gm)) {
  const en = unesc(m[2]);
  if (en.length >= min) out.push('[' + unesc(m[1]).slice(0, 46) + ']\n   ' + en);
}
console.log('长句 ' + out.length + ' 条：');
console.log(out.join('\n'));
