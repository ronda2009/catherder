// 抽取界面可见中文文案（给 i18n 词典用）。
// 用法: node tools/i18n-extract.cjs html|js [文件...]
// html: 取标签之间的纯文本 + title/placeholder/aria-label 属性值
// js  : 取含中文的字符串字面量
const fs = require('fs');
const mode = process.argv[2];
const files = process.argv.slice(3);
const CJK = /[\u4e00-\u9fff\u3000-\u303f\uff00-\uffef]/;
const out = new Map();
const add = (s) => {
  s = s.replace(/\s+/g, ' ').trim();
  if (!s || !CJK.test(s)) return;
  if (/^\/\/|^\*|^\/\*/.test(s)) return;
  out.set(s, (out.get(s) || 0) + 1);
};
for (const f of files) {
  const src = fs.readFileSync(f, 'utf8');
  if (mode === 'html') {
    const body = src.replace(/<!--[\s\S]*?-->/g, '');
    // 属性值
    for (const m of body.matchAll(/(title|placeholder|aria-label)\s*=\s*"([^"]*)"/g)) add(m[2]);
    // 标签之间的文本
    for (const seg of body.split(/<[^>]+>/)) add(seg);
  } else {
    for (const m of src.matchAll(/(['"`])((?:[^'"`\\]|\\.)*?)\1/g)) {
      let s = m[2];
      if (!s.includes('${') && !s.includes('$')) { /* 保留模板，后面单独看 */ }
      s = s.replace(/\\"/g, '"').replace(/\\'/g, "'").replace(/\\n/g, ' ');
      add(s);
    }
  }
}
const list = [...out.keys()].sort();
const path = 'D:/06.AItools/openclaw/workspace/tmp/data/i18n-' + mode + '.json';
fs.writeFileSync(path, JSON.stringify(list, null, 1));
console.log('条数=' + list.length + '  已写 ' + path);
