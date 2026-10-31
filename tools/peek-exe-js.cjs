// 从旧 exe 里挖出前端 JS 的原文片段（exe 里 go:embed 存的是原文字节），用于核对「改回来」的原始文案。
// 用法：node tools\peek-exe-js.cjs <exe路径> 上移
const fs = require('fs');
const exe = process.argv[2];
const needle = process.argv[3] || '上移';
const buf = fs.readFileSync(exe);
const s = buf.toString('utf8');
let i = -1, n = 0;
while ((i = s.indexOf(needle, i + 1)) >= 0 && n < 6) {
  const a = Math.max(0, i - 260), b = Math.min(s.length, i + 260);
  console.log(`--- 命中 ${++n} @${i} ---`);
  console.log(s.slice(a, b).replace(/[^\x20-\x7e\u4e00-\u9fff（）「」·✅→←↑↓：，。、！？；『』]/g, '·'));
}
if (!n) console.log('没找到「' + needle + '」');
