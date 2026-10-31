// 英文译文体检（i18n-check.cjs 只管「漏没漏翻」，这个管「翻出来的英文对不对」）。
// 用法：node tools\i18n-lint.cjs            # 全部问题
//      node tools\i18n-lint.cjs --only 显卡  # 只看含某词的条目
// 查六类：
//  A 译文里还夹着中文（＝没翻干净）
//  B 译文和中文一模一样（＝登记了但等于没翻）
//  C 首尾空格/标点不对齐（中文「版本 」带空格，英文也必须带，否则界面会粘字「Version1.0.1」）
//  D 括号/引号数量对不上（中文有「」英文要有引号；数量错＝句子结构被翻坏）
//  E 可疑英文（双空格、句中出现小写句首、明显直译词、单词里混大写等）
//  F 同一个中文词在不同条目里译法不一致（例：显卡一会儿 graphics card 一会儿 GPU）
const fs = require('node:fs');
const base = 'D:/06.AItools/openclaw/workspace/docs/项目/AI 代理 任务批发工具/app/frontend/dist/';
const unesc = (s) => s.replace(/\\'/g, "'").replace(/\\"/g, '"').replace(/\\\\/g, '\\');
const only = process.argv.includes('--only') ? process.argv[process.argv.indexOf('--only') + 1] : '';

const i18n = fs.readFileSync(base + 'i18n.js', 'utf8');
const dict = new Map();
for (const m of i18n.matchAll(/^\s*'((?:[^'\\]|\\.)*)'\s*:\s*'((?:[^'\\]|\\.)*)'/gm)) dict.set(unesc(m[1]), unesc(m[2]));
const rules = [];
for (const m of i18n.matchAll(/^\s*\[(\/.*?\/[a-z]*)\s*,\s*\(m[^)]*\)\s*=>\s*(`(?:[^`\\]|\\.)*`)/gm)) rules.push([m[1], unesc(m[2])]);

const cn = (s) => (s.match(/[一-鿿]/g) || []);
const lead = (s) => (s.match(/^\s+/) || [''])[0];
const trail = (s) => (s.match(/\s+$/) || [''])[0];
const issues = [];
const add = (kind, key, note) => { if (!only || key.includes(only) || kind.includes(only)) issues.push([kind, key, note]); };

for (const [zh, en] of dict) {
  if (cn(en).length) add('A 夹中文', zh, '译文里还有「' + cn(en).join('') + '」：' + en);
  if (en === zh) add('B 没翻', zh, '译文与中文完全相同');
  const lz = lead(zh), rz = lead(en), tz = trail(zh), rt = trail(en);
  if (tz !== rt && !(/[：；，、！？（）]$/.test(zh.trim()) && rt === ' ' && tz === '') && !(zh.trim().endsWith('第') && rt === '' && /[#0-9]$/.test(en.trim()))) add('C 尾部空格', zh, '中文带 ' + JSON.stringify(tz) + ' 英文带 ' + JSON.stringify(rt) + ' → 会粘字/多空');
  // 首部：英文片段接在别的文字后面本来就要带空格（「派工范围。」→' from the dispatch scope.'）；
  // 中文「30 秒前」的空格是隔数字与单位，英文 '30s ago' 本来就连写 — 都不算错。
  if (lz !== rz && !(lz === '' && rz === ' ') && !(lz === ' ' && rz === '')) add('C 首部空格(要人工看)', zh, '中文带 ' + JSON.stringify(lz) + ' 英文带 ' + JSON.stringify(rz) + '：' + en);
  const pairs = [['（', '('], ['）', ')']];
  const enClean = en.replace(/\((?:s|es|ed|ing)\)/g, ''); // 英文复数/时态写法 file(s)、item(s)、task(s) 不是括号，别误报
  for (const [a, b] of pairs) {
    const nz = zh.split(a).length - 1, ne = enClean.split(b).length - 1;
    // 括号位置可以换（中文把补充说明放括号里、英文改成句子后面）；
    // 中文本来没括号而英文自己加了一对（如「按 Ctrl+Enter 也一样」→ (Ctrl+Enter does the same)）＝正常写法。
    // 只查**中文有括号而英文对不上**（那才是把内容丢了）。
    if (nz > 0 && nz !== ne) add('D 括号总数', zh, '中文 ' + nz + ' 个 ' + a + '，英文 ' + ne + ' 个 ' + b + '：' + en);
  }
  // 「」在英文里用引号，但形式不限（"…" / '…' / “…”），只查有没有引号，不比数量。
  if ((zh.includes('「') || zh.includes('」')) && !/["'“”]/.test(en)) add('D 引号', zh, '中文有「」但英文没引号：' + en);
  if (zh.includes('（') && en.includes('（')) add('D 全角括号', zh, '英文里还留全角括号：' + en);
  if (/  /.test(en)) add('E 双空格', zh, en);
  if (/[a-z][A-Z]{2,}/.test(en.replace(/^(GPU|CSV|PNG|HTTP|API|OK|ID|PDF|CPU|RAM|Windows|Mac|Linux|NVIDIA|DPI|IME|Ctrl|Enter)/, '')))
    add('E 单词大小写', zh, en);
  if (/\bthe the\b|\ba a\b|\bof of\b|\bto to\b/.test(en)) add('E 重复词', zh, en);
  // 英文里残留全角标点（，。、：；！？「」｜）＝真错，界面上一眼能看出来。
  const fullwidth = en.match(/[，。、：；！？「」｜／（）]/g);
  if (fullwidth) add('G 英文里残留全角标点', zh, [...new Set(fullwidth)].join('') + ' → ' + en);
  // 整句译文却以小写开头（不是片段拼接用的短词条）＝看着像没写完。
  if (/^[a-z]/.test(en.trim()) && en.trim().length > 28 && !/^(move|opened|edited|save|press|tick|checked|dispatch|edit|one|all|no|the|window|task|item|file|row|screenshot|exported|added|done|error|gateway|backend|watching|living|holding|waiting|retrying|reading|sending|removed|cleared|discarded)/.test(en.trim())) {
    // 接在前缀后面的原因（如「自动关机已撤销：不在设定的关机时段」）小写开头是对的。
    const suffixReason = ['不在设定的关机时段', '还有任务标着异常，没处理', '仍有任务未完成', '未勾选任何可派工窗口', '读任务数失败', '等条件满足', '用户在界面点击取消', '模型服务还在忙'];
    if (!suffixReason.includes(zh)) add('H 整句小写开头', zh, en);
  }
  if (/[,;]$/.test(en.trim()) && !/[,;]$/.test(zh.trim())) add('D 句尾多标点', zh, en);
  if (/[，；：]$/.test(zh.trim()) && !/[,;:]$/.test(en.trim())) add('D 句尾标点', zh, '中文以全角标点收尾，英文没对应收尾：' + en);
}

// F：同一中文词在多条里出现，英文译法却不止一种。
const term = { 显卡: ['graphics card', 'GPU'], 窗口: ['window'], 队列: ['queue', 'queued'], 阈值: ['threshold'] };
for (const [zhWord, expect] of Object.entries(term)) {
  const hits = [...dict].filter(([z]) => z.includes(zhWord));
  const seen = new Map();
  for (const [z, e] of hits) {
    for (const w of expect) {
      const re = new RegExp('\\b' + w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '\\b', 'i');
      if (re.test(e)) { if (!seen.has(w)) seen.set(w, []); seen.get(w).push(z); }
    }
  }
  if (expect.length > 1 && seen.size > 1) {
    const kinds = [...seen.keys()];
    add('F 译法不统一', zhWord, '同一个词出现 ' + kinds.join(' / ') + ' 两种译法：' + kinds.map((k) => seen.get(k)[0]).join(' ｜ '));
  }
}

// 整句规则（带数字/窗口名的日志句）也过一遍 A/C/E。
for (const [re, tpl] of rules) {
  const body = tpl.replace(/\$\{[^}]*\}/g, 'X');
  if (cn(body).length) add('A 夹中文(规则)', re, body);
  if (/  /.test(body)) add('E 双空格(规则)', re, body);
  if (body.includes('（') || body.includes('」')) add('D 全角括号(规则)', re, body);
}

// I：悬停提示（title）与独立提示语要句首大写（本程序其余提示都是句首大写）。
// 从 app.js / shot.js / index.html 里抓 title="…" 与 feed('…') 用的中文串，再查词典里的英文开头。
const uiSrc = ['app.js', 'shot.js', 'index.html'].map((f) => fs.readFileSync(base + f, 'utf8')).join('\n');
const titleKeys = new Set();
for (const m of uiSrc.matchAll(/title="[^"]*\$\{tr\('((?:[^'\\]|\\.)*)'\)/g)) titleKeys.add(unesc(m[1]));
for (const m of uiSrc.matchAll(/title="([^"]*[一-鿿][^"]*)"/g)) titleKeys.add(m[1]);
// 独立显示的说明文字（<span>…</span> 里只有 tr('…')）也算整句。
for (const m of uiSrc.matchAll(/<span[^>]*>\s*\$\{tr\('((?:[^'\\]|\\.)*)'\)\}\s*<\/span>/g)) titleKeys.add(unesc(m[1]));
// 接在任务号后面的提示（feed(`T-001 ` + tr('…'))）＝后半句，英文小写开头是对的，不能算整句。
const suffixKeys = new Set();
for (const m of uiSrc.matchAll(/\+\s*tr\('((?:[^'\\]|\\.)*)'\)/g)) suffixKeys.add(unesc(m[1]));
for (const k of suffixKeys) titleKeys.delete(k);
for (const [zh, en] of dict) {
  if (!titleKeys.has(zh)) continue;
  const t = en.trim();
  // 接在别的文字后面的片段（前导空格）与带 → 的箭头项不算独立句。
  if (/^[a-z]/.test(t) && !/^\s/.test(en)) add('I 提示语小写开头', zh, en);
}

console.log('词典 ' + dict.size + ' 条 + 整句规则 ' + rules.length + ' 条 → 问题 ' + issues.length + ' 处');
const byKind = new Map();
for (const [k, key, note] of issues) { if (!byKind.has(k)) byKind.set(k, []); byKind.get(k).push([key, note]); }
for (const [k, list] of [...byKind].sort()) {
  console.log('\n=== ' + k + '（' + list.length + '） ===');
  for (const [key, note] of list.slice(0, 25)) console.log('  [' + key.slice(0, 60) + '] ' + note.slice(0, 150));
}
