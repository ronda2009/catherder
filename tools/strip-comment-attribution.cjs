// 注释清理第二遍：把「老板/我/上一版/日期」这类归因从中性视角改写，只留「这行是什么意思、为什么这么干」。
// 逐行扫描时区分字符串与注释（URL 里的 // 不会被当注释）；只改注释文字，代码部分原样保留。
// 用法：node tools/strip-comment-attribution.cjs [--apply]
const fs = require('fs');
const path = require('path');

const apply = process.argv.includes('--apply');
const ROOT = path.join(__dirname, '..', 'app');

function walk(dir, out = []) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    if (e.name === 'bin' || e.name === 'node_modules' || e.name.startsWith('.')) continue;
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walk(p, out);
    else if (/\.(go|js)$/.test(e.name)) out.push(p);
  }
  return out;
}

// 把一行拆成「代码 + 注释」，带块注释状态；字符串与 URL 里的 // 不当注释。
function splitLine(line, state) {
  let i = 0;
  const n = line.length;
  if (state.block) {
    const end = line.indexOf('*/');
    if (end < 0) return { code: '', mark: null, comment: line, tail: '', block: true };
    return { code: '', mark: null, comment: line.slice(0, end + 2), tail: line.slice(end + 2), block: false };
  }
  while (i < n) {
    const c = line[i], nx = line[i + 1];
    if (c === '"' || c === "'" || c === '`') {
      const q = c; i++;
      while (i < n) { if (line[i] === '\\') { i += 2; continue; } if (line[i] === q) { i++; break; } i++; }
      continue;
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
const RULES = [
  [new RegExp('按用户\\s*' + D + '\\s*定的', 'g'), '默认档位'],
  [new RegExp('用户\\s*' + D + '[：:]', 'g'), ''],
  [new RegExp('（用户[^（）]*[：:][^（）]*）', 'g'), ''],
  [/（[^（）]*(上一版|上轮|之前|后来|一开始|自检抓到)[^（）]*）/g, ''],
  [/\([^()]*(上一版|上轮|之前|自检抓到|20\d\d-\d\d-\d\d)[^()]*\)/g, ''],
  [new RegExp('（[^（）]*' + D + '[^（）]*）', 'g'), ''],
  [new RegExp('[，、]?\\s*' + D + '\\s*(报|定|点头|反馈|要求|问)?[：:]?', 'g'), ''],
  [/（例：[^（）]*）/g, ''],
  [/就是老板报的/g, '就是'],
  [/老板报的/g, ''],
  [/老板/g, '用户'],
  [/我开着测试的/g, '开发调试用的'],
  [/用户和我都能一眼确认/g, '使用者一眼就能确认'],
  [/（避免「改了但看不到变化」的争议）/g, '（避免「改了却看不出变化」）'],
  [/上一版拿黑色描边当裁剪区，黑色露出来就是用户看到的「黑框」——不要描边。/g, '描边会被当成裁剪区露出来，看着像黑框——不要描边。'],
  [/上轮就是漏了这个，/g, '漏了这步就会：'],
  [/（用户报的「卡在取色器上」：/g, '（「卡在取色器上」：/'],
  [/原来写死一个号/g, '原先写死一个号'],
  [/按钮名按用户的要求改回「上移 \/ 下移」/g, '按钮名用「上移 / 下移」'],
  [/用户报的「绘图员h 被误判闲置」就是这么来的/g, '误判闲置就是这么来的'],
  [/用户报的「绘图员h 被误判闲置」就是这种/g, '误判闲置的典型情形'],
  [/就是用户报的「闪一下/g, '就是「闪一下/g'],
  [/正在就地修改的那条任务：算「还没完」，不能趁用户改的时候判定队列空去关机/g, '正在就地修改的那条算「还没完」：用户编辑期间不判队列空去关机'],
  [/别在用户改的时候判定队列空去关机/g, '用户编辑期间不判队列空去关机'],
  [/用户正在界面里改着、随手跑一条/g, '用户正在界面里改着时，随手跑一条'],
  [/用户问「飞Q 为何等了二十几分钟」，原来日志里看不出来/g, '日志里要能看出每条为什么在等'],
  [/状态里缺一个已完成的绿色/g, '状态栏要有已完成的计数'],
  [/完成的任务不许自动不见/g, '完成的任务不许自动消失'],
  [/两份并存时靠它区分哪个是用户挂后台的、哪个是开发调试用的/g, '两份并存时靠它区分正式在跑的与开发调试用的'],
  [/同时跑两份（用户挂后台用 \+ 开发调试用的）/g, '同时跑两份（正式挂后台 + 开发调试）'],
  [/任务栏\/窗口图标变「出错」样（用户 2026-10-24 反馈）/g, '任务栏/窗口图标变「出错」样'],
  [/\t+/g, ' '],
];

function tidy(t) {
  let s = t;
  const hadEmptyParen = /（\s*）|\(\s*\)/.test(s);
  for (const [re, rep] of RULES) s = s.replace(re, rep);
  if (!hadEmptyParen) s = s.replace(/[（(]\s*[)）]/g, '');
  s = s.replace(/[，、;；]{2,}/g, '，');
  s = s.replace(/[ \t]{2,}/g, ' ');
  s = s.replace(/^[\s，、;：]+/, '').replace(/[\s，、;]+$/, '');
  return s;
}

const files = walk(ROOT);
const diffs = [];
let changed = 0;

for (const f of files) {
  const before = fs.readFileSync(f, 'utf8');
  const lines = before.split('\n');
  let block = false;
  const out = lines.map((raw) => {
    const r = splitLine(raw, { block });
    block = r.block;
    if (!r.mark && !r.comment) return raw;
    if (r.mark === null) {
      const m = raw.match(/^(\s*\*\s?)(.*)$/);
      if (!m) return raw;
      const t = tidy(m[2]);
      return t === m[2] ? raw : m[1] + t;
    }
    if (/^\s*(go:|nolint|lint:|source:|Copyright|SPDX)/.test(r.comment)) return raw;
    const t = tidy(r.comment);
    if (t === r.comment) return raw;
    if (t.trim() === '') return r.code.replace(/\s+$/, '');
    const closed = r.mark === '/*' && !r.block;
    return `${r.code}${r.mark === '//' ? '// ' : '/* '}${t}${closed ? ' */' : ''}${r.tail}`;
  });
  const after = out.join('\n');
  if (after === before) continue;
  const rel = path.relative(ROOT, f);
  changed++;
  const bl = before.split('\n'), al = after.split('\n');
  for (let i = 0; i < Math.max(bl.length, al.length); i++) {
    if (bl[i] !== al[i]) { diffs.push(`  ${rel} L${i + 1}\n    改前: ${(bl[i] || '').trim()}\n    改后: ${(al[i] || '').trim()}`); break; }
  }
  if (apply) fs.writeFileSync(f, after, 'utf8');
}
console.log(`${apply ? '已写回' : '试跑'}：改 ${changed} 个文件`);
console.log(diffs.slice(0, 80).join('\n'));
