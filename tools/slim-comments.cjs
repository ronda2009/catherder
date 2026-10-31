// 注释瘦身：只动注释，不动代码。
// 规则（老板 2026-10-25：注释太多一看太 AI；台账里都记了，注释别重复）：
//   1) 删掉「（用户 2026-10-25）」「（老板 2026-10-24 提）」这类日期归因
//   2) 删掉「定稿 §6」这类文档编号引用
//   3) 删掉「实测踩过」「实测」这类过程故事
//   4) 注释里连续多行讲同一个「为什么」→ 只留第一行
// 用法：node slim-comments.cjs [--apply]   不带 --apply 只报会改多少行。
const fs = require('fs');
const path = require('path');

const APPLY = process.argv.includes('--apply');
const ROOT = 'D:\\06.AItools\\openclaw\\workspace\\docs\\项目\\AI 代理 任务批发工具\\app';
const FILES = [
  'internal\\engine\\engine.go', 'internal\\engine\\confirm.go', 'internal\\engine\\order_test.go',
  'internal\\app\\app.go', 'internal\\app\\tray_windows.go', 'internal\\app\\screenshot.go',
  'internal\\store\\store.go', 'internal\\openclaw\\ws.go', 'internal\\openclaw\\config.go',
  'internal\\screen\\grab_windows.go', 'internal\\cli\\run.go', 'main.go',
  'frontend\\dist\\app.js', 'frontend\\dist\\shot.js', 'frontend\\dist\\app.css', 'frontend\\dist\\index.html',
];

// 只在注释语境里做替换，避免动到字符串常量。
function slimLine(line) {
  const t = line.trimStart();
  const isComment = t.startsWith('//') || t.startsWith('/*') || t.startsWith('*') || t.startsWith('<!--');
  if (!isComment) return line;
  let s = line;
  // 日期归因（含在括号里的整段）
  s = s.replace(/[（(][^（()）]*?(?:用户|老板)\s*20\d\d-\d\d-\d\d[^（()）]*?[)）]/g, '');
  s = s.replace(/(?:用户|老板)\s*20\d\d-\d\d-\d\d\s*(?:要|提|说|反馈|立|定)?[:：]?\s*/g, '');
  // 文档编号
  s = s.replace(/[（(]定稿\s*§[^（()）]*?[)）]/g, '').replace(/定稿\s*§\s*\d+(?:\s*[、,]\s*§?\d+)*/g, '');
  // 过程故事
  s = s.replace(/[（(][^（()）]*?实测[^（()）]*?[)）]/g, '');
  s = s.replace(/实测(?:踩过|过|通过|成功|发现|返回)?[，,。]?\s*/g, '');
  // 收尾：把「。。」「，。」「 。」收拾干净
  s = s.replace(/，，+/g, '，').replace(/([。，])\s*。\s*$/g, '。').replace(/\s{2,}(?=。)/g, ' ');
  // 空注释（只剩 // 或 // ）
  if (/^\s*\/\/\s*$/.test(s)) return null;
  if (/^\s*\/\/\s*[，。、：;·-]+\s*$/.test(s)) return null;
  return s.replace(/[ \t]+$/g, '');
}

let total = 0;
for (const rel of FILES) {
  const p = path.join(ROOT, rel);
  if (!fs.existsSync(p)) continue;
  const lines = fs.readFileSync(p, 'utf8').split(/\r?\n/);
  const out = [];
  let prevComment = false, changed = 0;
  for (const l of lines) {
    const r = slimLine(l);
    if (r === null) { changed++; continue; }
    if (r !== l) changed++;
    const t = r.trimStart();
    const isC = t.startsWith('//') || t.startsWith('/*') || t.startsWith('*');
    // 连续两行以上讲「为什么」的注释：只留首行（首行通常已含结论）
    if (isC && prevComment && /^(\/\/|\/?\*)\s*(为什么|注：|注意：)/.test(t)) { changed++; continue; }
    prevComment = isC;
    out.push(r);
  }
  if (changed) {
    total += changed;
    console.log(rel + '  改动 ' + changed + ' 行（' + lines.length + ' → ' + out.length + '）');
    if (APPLY) fs.writeFileSync(p, out.join('\n'), 'utf8');
  }
}
console.log((APPLY ? '已应用，' : '预演，') + '共 ' + total + ' 行');
