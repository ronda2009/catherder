// 汇总 Go 侧「写进事件日志」的中文文案（.Log / logEvent 调用里的字符串字面量）。
// 日志页在英文界面下要不要翻，靠这份清单登记。临时排查用。
const fs = require('fs');
const path = require('path');
const root = path.join(__dirname, '..', 'app', 'internal');
const out = new Map();
function walk(dir) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) { walk(p); continue; }
    if (!e.name.endsWith('.go') || e.name.endsWith('_test.go')) continue;
    const src = fs.readFileSync(p, 'utf8').split(/\r?\n/);
    src.forEach((line, i) => {
      if (!/\.(Log|logEvent)\(/.test(line)) return;
      const strs = line.match(/"(?:[^"\\]|\\.)*"/g) || [];
      for (const s of strs) {
        const t = s.slice(1, -1);
        if (!/[\u4e00-\u9fff]/.test(t)) continue;
        if (!out.has(t)) out.set(t, e.name + ' L' + (i + 1));
      }
    });
  }
}
walk(root);
console.log('日志文案（去重）' + out.size + ' 条');
for (const [t, at] of out) console.log('  ' + at + ' :: ' + t.slice(0, 110));
fs.writeFileSync(path.join(__dirname, '..', '..', '..', '..', 'tmp', 'data', 'i18n-log.txt'),
  [...out].map(([t, at]) => at + '\t' + t).join('\n'), 'utf8');
