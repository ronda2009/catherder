// 打印 app.js 指定行的原文（带可见缩进），改代码前先看清缩进，别靠猜。
// 用法：node tools\peek-js-lines.cjs app/frontend/dist/app.js 388 392
const fs = require('fs');
const f = process.argv[2];
const a = Number(process.argv[3]), b = Number(process.argv[4] || process.argv[3]);
const lines = fs.readFileSync(f, 'utf8').split('\n');
for (let i = a; i <= b && i <= lines.length; i++) console.log(i + ': ' + JSON.stringify(lines[i - 1]));
