// 发布前注释清理：把注释里的日期归因、过程故事、开发日志腔去掉，只留「这行是什么意思 / 为什么这么干」。
// 只动注释，不动代码：跑完会用「去掉注释后的内容」逐文件比对，不一致就拒绝写回。
// 用法：node tools/strip-comment-story.cjs [--apply]
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const apply = process.argv.includes('--apply');
const ROOT = path.join(__dirname, '..', 'app');

// 归因/故事标记：出现这些词的注释内容要瘦身
const ATTR = /(用户|老板|我|本轮|本次|之前|后来|一开始|自检抓到|实测过|按老板|老板定|老板点头|老板纠正|T-\d{3})/;
const DATE = /20\d{2}-\d{2}-\d{2}/;

function walk(dir, out = []) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    if (e.name === 'bin' || e.name === 'node_modules' || e.name.startsWith('.')) continue;
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walk(p, out);
    else if (/\.(go|js)$/.test(e.name)) out.push(p);
  }
  return out;
}

// 去注释（用于「代码没变」的自校）：整行注释与行尾注释都去掉，粗但前后同一算法即可比对。
function stripComments(src) {
  return src
    .split('\n')
    .map((l) => {
      let out = '';
      let i = 0;
      let inBlock = false;
      let inStr = null;
      while (i < l.length) {
        const c = l[i], n = l[i + 1];
        if (inBlock) { if (c === '*' && n === '/') { inBlock = false; i += 2; continue; } i++; continue; }
        if (inStr) { if (c === '\\') { i += 2; continue; } if (c === inStr) inStr = null; i++; continue; }
        if (c === '"' || c === "'" || c === '`') { inStr = c; out += c; i++; continue; }
        if (c === '/' && n === '/') break;
        if (c === '/' && n === '*') { inBlock = true; i += 2; continue; }
        out += c; i++;
      }
      return out.replace(/\s+$/, '');
    })
    .join('\n');
}

// 注释瘦身：删掉带归因/日期的括号补充；整行只是故事的就删行。
function cleanCommentText(t) {
  let s = t;
  s = s.replace(/（[^（）]*(?:用户|老板|我|本轮|自检|实测|按老板|T-\d{3}|20\d{2}-\d{2}-\d{2})[^（）]*）/g, '');
  s = s.replace(/\((?:[^()]*(?:用户|老板|本轮|self[- ]?check|20\d{2}-\d{2}-\d{2})[^()]*)\)/g, '');
  s = s.replace(/[，、;；]\s*(?:用户|老板)\s*20\d{2}-\d{2}-\d{2}[^，。;；]*/g, '');
  s = s.replace(/\s{2,}/g, ' ').replace(/[，、;；]\s*([：:])/g, '$1');
  s = s.replace(/^[\s，、;：]+/, '').replace(/[\s，、;：]+$/, '');
  return s;
}

const files = walk(ROOT);
let changed = 0, linesCut = 0, linesSlim = 0;
const report = [];

for (const f of files) {
  const before = fs.readFileSync(f, 'utf8');
  const lines = before.split('\n');
  const out = [];
  for (let i = 0; i < lines.length; i++) {
    const raw = lines[i];
    const m = raw.match(/^(\s*)\/\/(.*)$/);
    if (m && !/^\s*\/\/\s*(go:|nolint|lint:|source:|third_party|Copyright|SPDX)/i.test(raw)) {
      const body = m[2];
      const isBlockDoc = body.trimStart().startsWith('/') || body.trim() === '';
      const text = cleanCommentText(body);
      const storyOnly = ATTR.test(body) && (DATE.test(body) || /本轮|之前|后来|一开始|自检抓到|老板纠正|按老板|老板点头/.test(body)) && text.length < 4;
      if (!isBlockDoc && text.trim() === '' && !storyOnly && body.trim() !== '') { out.push(raw); continue; }
      if (text.trim() === '') { linesCut++; continue; }
      if (text !== body) linesSlim++;
      out.push(m[1] + '//' + (text.startsWith(' ') ? text : ' ' + text));
      continue;
    }
    // 行尾注释
    const t = raw.match(/^(.*?\S)\s+\/\/(.*)$/);
    if (t && !/["'`]/.test(t[1].slice(-1)) && !/^\s*\/\//.test(raw)) {
      const text = cleanCommentText(t[2]);
      if (text.trim() === '') { linesCut++; out.push(t[1]); continue; }
      if (text !== t[2]) { linesSlim++; out.push(t[1] + ' // ' + text); continue; }
    }
    out.push(raw);
  }
  let after = out.join('\n');
  after = after.replace(/\n{3,}/g, '\n\n');
  if (after === before) continue;
  const same = stripComments(before) === stripComments(after);
  const rel = path.relative(ROOT, f);
  if (!same) { report.push(`  跳过（去注释后内容不一致，需手工改）：${rel}`); continue; }
  changed++;
  report.push(`  ${rel}`);
  if (apply) fs.writeFileSync(f, after, 'utf8');
}
console.log(`${apply ? '已写回' : '试跑（未写回）'}：改 ${changed} 个文件，删注释 ${linesCut} 行，瘦身 ${linesSlim} 行`);
console.log(report.slice(0, 60).join('\n'));
