// 注释瘦身：只动注释，不动代码。
// 规矩（老板定的：注释别 AI 味、别什么都注释；原因归台账与脚本索引）：
//   1) 删日期归因（「（用户 2026-10-25）」「老板 2026-10-24 提」）与文档编号（「定稿 §6」）
//   2) 删过程故事（「实测踩过」「上一版」「上轮就是漏了这个」）
//   3) 第一人称与「老板报的」改成中性说法（「我开着测试的」→「开发调试用的」）
//   4) 连续注释里重复讲同一个「为什么」→ 只留首行
// 逐行区分字符串与注释（URL 里的 // 不当注释），代码部分原样保留；可反复跑，跑干净后报「改 0 个文件」。
// 改完必跑编译 + 测试（注释删多了会破坏语法，尤其块注释）。
// 用法：node tools/slim-comments.cjs [--apply]
const fs = require('fs');
const path = require('path');

const APPLY = process.argv.includes('--apply');
const ROOT = path.join(__dirname, '..', 'app');
const EXT = /\.(go|js|css|html)$/;

function walk(dir, out = []) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    if (e.name === 'bin' || e.name === 'node_modules' || e.name.startsWith('.')) continue;
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walk(p, out);
    else if (EXT.test(e.name)) out.push(p);
  }
  return out;
}

// 把一行拆成「代码 + 注释」，带块注释状态；字符串里的 // 不当注释。
function splitLine(line, state) {
  let i = 0;
  const n = line.length;
  if (state.block) {
    const end = line.indexOf('*/');
    if (end < 0) return { code: '', mark: null, comment: line, tail: '', block: true };
    return { code: '', mark: null, comment: line.slice(0, end + 2), tail: line.slice(end + 2), block: false };
  }
  if (state.html) {
    const end = line.indexOf('-->');
    if (end < 0) return { code: '', mark: null, comment: line, tail: '', block: false, html: true };
    return { code: '', mark: null, comment: line.slice(0, end + 3), tail: line.slice(end + 3), block: false, html: false };
  }
  while (i < n) {
    const c = line[i], nx = line[i + 1];
    if (c === '"' || c === "'" || c === '`') {
      const q = c; i++;
      while (i < n) { if (line[i] === '\\') { i += 2; continue; } if (line[i] === q) { i++; break; } i++; }
      continue;
    }
    if (c === '<' && line.slice(i, i + 4) === '<!--') {
      const rest = line.slice(i + 4);
      const end = rest.indexOf('-->');
      if (end < 0) return { code: line.slice(0, i), mark: '<!--', comment: rest, tail: '', block: false, html: true };
      return { code: line.slice(0, i), mark: '<!--', comment: rest.slice(0, end), tail: rest.slice(end + 3), block: false, html: false };
    }
    if (c === '/' && nx === '/') return { code: line.slice(0, i), mark: '//', comment: line.slice(i + 2), tail: '', block: false };
    if (c === '/' && nx === '*') {
      const rest = line.slice(i + 2);
      const end = rest.indexOf('*/');
      if (end < 0) return { code: line.slice(0, i), mark: '/*', comment: rest, tail: '', block: true };
      return { code: line.slice(0, i), mark: '/*', comment: rest.slice(0, end), tail: rest.slice(end + 2), block: false };
    }
    i++;
  }
  return { code: line, mark: null, comment: '', tail: '', block: false };
}

const D = '20\\d{2}-\\d{2}-\\d{2}';
// 顺序要紧：先处理带日期的整段，再处理裸日期，最后做中性改写。
const RULES = [
  [new RegExp('（[^（）]*(用户|老板)[^（）]*' + D + '[^（）]*）', 'g'), ''],
  [new RegExp('\\([^()]*(user|owner)\\s*' + D + '[^()]*\\)', 'g'), ''],
  [new RegExp('按(用户|老板)\\s*' + D + '\\s*定的', 'g'), '默认档位'],
  [new RegExp('(用户|老板)\\s*' + D + '[：:]', 'g'), ''],
  [new RegExp('（[^（）]*' + D + '[^（）]*）', 'g'), ''],
  [/\([^()]*(上一版|上轮|自检抓到|20\d\d-\d\d-\d\d)[^()]*\)/g, ''],
  [new RegExp('[，、]?\\s*' + D + '\\s*(报|定|点头|反馈|要求|问|提)?[：:]?', 'g'), ''],
  [/（[^（）]*(上一版|上轮|之前|后来|一开始|自检抓到|实测过|实测踩过)[^（）]*）/g, ''],
  [/实测(?:踩过|过|通过|成功|发现)[，,]?\s*/g, ''],
  [/(?:见|参见|详见)\s*(?:设计)?定稿\s*§\s*\d+(?:\s*[、,]\s*§?\s*\d+)*/g, ''],
  [/（[^（）]*§[^（）]*）/g, ''],
  [/(设计)?定稿\s*§\s*\d+(?:\s*[、,]\s*§?\s*\d+)*/g, ''],
  [/（例：[^（）]*）/g, ''],
  [/就是(用户|老板)报的/g, '就是'],
  [/(用户|老板)报的/g, ''],
  [/老板/g, '用户'],
  [/我开着测试的/g, '开发调试用的'],
  [/用户和我都能一眼确认/g, '使用者一眼就能确认'],
  [/（避免「改了但看不到变化」的争议）/g, '（避免「改了却看不出变化」）'],
  [/上轮就是漏了这个，/g, '漏了这步就会：'],
  [/原来写死一个号/g, '原先写死一个号'],
  [/正在就地修改的那条任务：算「还没完」，不能趁用户改的时候判定队列空去关机/g, '正在就地修改的那条算「还没完」：用户编辑期间不判队列空去关机'],
  [/别在用户改的时候判定队列空去关机/g, '用户编辑期间不判队列空去关机'],
  [/两份并存时靠它区分哪个是用户挂后台的、哪个是开发调试用的/g, '两份并存时靠它区分正式在跑的与开发调试用的'],
  [/用户正在界面里改着、随手跑一条/g, '用户正在界面里改着时，随手跑一条'],
  [/状态里缺一个已完成的绿色/g, '状态栏要有已完成的计数'],
  [/完成的任务不许自动不见/g, '完成的任务不许自动消失'],
  [/\t+/g, ' '],
];

function tidy(t) {
  let s = t;
  const hadEmptyParen = /（\s*）|\(\s*\)/.test(s);
  for (const [re, rep] of RULES) s = s.replace(re, rep);
  if (!hadEmptyParen) s = s.replace(/[（(]\s*[)）]/g, '');
  s = s.replace(/[，、;；]{2,}/g, '，');
  s = s.replace(/。{2,}/g, '。');
  s = s.replace(/[ \t]{2,}/g, ' ');
  s = s.replace(/^[\s，、;：]+/, '').replace(/[\s，、;]+$/, '');
  return s;
}

const files = walk(ROOT);
const diffs = [];
let changed = 0, cut = 0, slim = 0;

for (const f of files) {
  const before = fs.readFileSync(f, 'utf8');
  const lines = before.split('\n');
  const state = { block: false, html: false };
  const out = [];
  let prevComment = false;
  for (const raw of lines) {
    const r = splitLine(raw, state);
    state.block = r.block; state.html = r.html;
    if (!r.mark && !r.comment) { out.push(raw); prevComment = false; continue; }
    let text = null, keep = raw;
    if (r.mark === null) {
      const m = raw.match(/^(\s*\*\s?)(.*)$/);
      if (!m) { out.push(raw); continue; }
      text = m[2]; keep = (t) => m[1] + t;
    } else {
      if (/^\s*(go:|nolint|lint:|source:|Copyright|SPDX)/.test(r.comment)) { out.push(raw); continue; }
      text = r.comment;
      keep = (t) => {
        if (r.mark === '<!--') return `${r.code}<!-- ${t} -->${r.tail}`;
        const closed = r.mark === '/*' && !r.block;
        return `${r.code}${r.mark === '//' ? '// ' : '/* '}${t}${closed ? ' */' : ''}${r.tail}`;
      };
    }
    const t = tidy(text);
    const orig = text.replace(/^\s+/, '');
    if (t === orig) { prevComment = true; out.push(raw); continue; }
    if (t.trim() === '') { prevComment = false; continue; }
    // 连续注释里重复讲同一个「为什么」：只留首行
    if (prevComment && /^(为什么|注：|注意：)/.test(t)) { continue; }
    prevComment = true; out.push(keep(t));
  }
  const after = out.join('\n');
  if (after === before) continue;
  const rel = path.relative(ROOT, f);
  changed++;
  const bl = before.split('\n'), al = after.split('\n');
  let cutHere = 0, slimHere = 0;
  for (let i = 0; i < Math.max(bl.length, al.length); i++) {
    if (bl[i] === al[i]) continue;
    if (al[i] === undefined) { cutHere++; continue; }
    slimHere++;
    diffs.push(`  ${rel} L${i + 1}\n    改前: ${(bl[i] || '').trim()}\n    改后: ${(al[i] || '').trim()}`);
    break;
  }
  cut += cutHere; slim += slimHere;
  if (APPLY) fs.writeFileSync(f, after, 'utf8');
}
console.log(`${APPLY ? '已写回' : '预演'}：改 ${changed} 个文件（删注释 ${cut} 行、瘦身 ${slim} 行）`);
console.log(diffs.slice(0, 40).join('\n'));
