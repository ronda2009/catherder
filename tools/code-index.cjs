// 生成《代码索引.md》：列出每个源文件里的函数/关键元素 + 行号 + 它上面那行注释。
// 目的（老板 2026-10-25 提）：改东西先查索引 → 只读那一段 → 改完重跑本脚本更新索引，
// 不用每次把全部代码读一遍。
// 用法：node tools\code-index.cjs            （写到 代码索引.md）
//       node tools\code-index.cjs --check    （只报告有多少条目，不写文件）
const fs = require('fs');
const path = require('path');

const ROOT = path.resolve(__dirname, '..');
const APP = path.join(ROOT, 'app');
const OUT = path.join(ROOT, '代码索引.md');

// 要索引的位置：Go 源码 + 前端三件套 + 图标生成脚本。
const TARGETS = [
  { rel: 'main.go', kind: 'go' },
  { rel: 'internal/cli/run.go', kind: 'go' },
  { rel: 'internal/app/app.go', kind: 'go' },
  { rel: 'internal/app/export.go', kind: 'go' },
  { rel: 'internal/app/tray_windows.go', kind: 'go' },
  { rel: 'internal/app/winicon_windows.go', kind: 'go' },
  { rel: 'internal/app/autostart_windows.go', kind: 'go' },
  { rel: 'internal/engine/engine.go', kind: 'go' },
  { rel: 'internal/engine/confirm.go', kind: 'go' },
  { rel: 'internal/engine/shutdown.go', kind: 'go' },
  { rel: 'internal/store/store.go', kind: 'go' },
  { rel: 'internal/store/export.go', kind: 'go' },
  { rel: 'internal/openclaw/config.go', kind: 'go' },
  { rel: 'internal/openclaw/db.go', kind: 'go' },
  { rel: 'internal/openclaw/ws.go', kind: 'go' },
  { rel: 'frontend/dist/index.html', kind: 'html' },
  { rel: 'frontend/dist/app.js', kind: 'js' },
  { rel: 'frontend/dist/app.css', kind: 'css' },
];

const TESTS = fs.readdirSync(APP, { recursive: true })
  .filter((f) => String(f).endsWith('_test.go'))
  .map((f) => String(f).replace(/\\/g, '/'));

function readLines(rel) {
  const p = path.join(APP, rel.replace(/\//g, path.sep));
  if (!fs.existsSync(p)) return null;
  return fs.readFileSync(p, 'utf8').split(/\r?\n/);
}

// 取某行上方最近的注释（跳过空行，最多回看 3 行）。
function noteAbove(lines, i) {
  for (let k = i - 1, seen = 0; k >= 0 && seen < 3; k--, seen++) {
    const t = lines[k].trim();
    if (!t) continue;
    if (t.startsWith('//') || t.startsWith('/*') || t.startsWith('*')) {
      return t.replace(/^\/\/+?\s*/, '').replace(/^\/\*+\s*/, '').replace(/\s*\*\/$/, '').slice(0, 90);
    }
    break;
  }
  return '';
}

function indexGo(lines) {
  const out = [];
  lines.forEach((l, i) => {
    const m = l.match(/^func (?:\((\w+) \*?\w+\)\s*)?(\w+)/);
    if (m) {
      out.push({ line: i + 1, name: (m[1] ? m[1] + '.' : '') + m[2], note: noteAbove(lines, i) });
      return;
    }
    const t = l.match(/^type (\w+)/);
    if (t) out.push({ line: i + 1, name: 'type ' + t[1], note: noteAbove(lines, i) });
  });
  return out;
}

function indexJs(lines) {
  const out = [];
  lines.forEach((l, i) => {
    const m = l.match(/^(?:async\s+)?function\s+(\w+)/);
    if (m) { out.push({ line: i + 1, name: m[1] + '()', note: noteAbove(lines, i) }); return; }
    const a = l.match(/^\$\('([\w-]+)'\)\.addEventListener\('(\w+)'/);
    if (a) out.push({ line: i + 1, name: `#${a[1]} → ${a[2]}`, note: noteAbove(lines, i) });
  });
  return out;
}

function indexHtml(lines) {
  const out = [];
  lines.forEach((l, i) => {
    const m = l.match(/id="([\w-]+)"/);
    if (m) out.push({ line: i + 1, name: '#' + m[1], note: (l.match(/>([^<>{}]{4,60})</) || [, ''])[1].trim() });
  });
  return out;
}

function indexCss(lines) {
  const out = [];
  lines.forEach((l, i) => {
    const m = l.match(/^([.#][\w .,>:\[\]-]+)\s*\{/);
    if (m) out.push({ line: i + 1, name: m[1].trim(), note: noteAbove(lines, i) });
  });
  return out;
}

const sections = [];
let total = 0;
for (const t of TARGETS) {
  const lines = readLines(t.rel);
  if (!lines) { sections.push(`## ${t.rel}\n\n（文件不存在）\n`); continue; }
  let items = [];
  if (t.kind === 'go') items = indexGo(lines);
  else if (t.kind === 'js') items = indexJs(lines);
  else if (t.kind === 'html') items = indexHtml(lines);
  else items = indexCss(lines);
  total += items.length;
  const rows = items.map((it) => `| ${it.line} | \`${it.name}\` | ${it.note.replace(/\|/g, '\\|')} |`).join('\n');
  sections.push(`## ${t.rel}（${lines.length} 行）\n\n| 行 | 名称 | 它是干什么的 |\n|---|---|---|\n${rows}\n`);
}

const testRows = TESTS.map((rel) => {
  const lines = readLines(rel) || [];
  const names = [];
  lines.forEach((l, i) => { const m = l.match(/^func (Test\w+)/); if (m) names.push(`\`${m[1]}\`(L${i + 1})`); });
  return `- \`${rel}\`：${names.join('、') || '（无测试函数）'}`;
}).join('\n');

const head = `# 代码索引（改哪儿先查这里）

> **怎么用**（老板 2026-10-25 提的做法）：要改/查某个功能 → 先在这份索引里找到那个函数在哪个文件第几行 → **只读那一段** → 改完跑
> \`node tools\\code-index.cjs\` 重新生成这份索引。**不用每次把全部代码读一遍。**
> 行号只是给你跳过去用的，**认函数名不认行号**（加了几行行号就会漂，函数名不会漂）。
> 生成时间：${new Date().toLocaleString('zh-CN', { hour12: false })}｜条目 ${total} 个｜文件 ${TARGETS.length} 个

## 一句话架构

- \`main.go\`：认命令行参数、决定用哪个库文件、编译期决定要不要界面/托盘，把 engine 交给界面或命令行。
- \`internal/engine\`：**所有业务判断**（判空闲、排队、派发、判完成、闸门、关机、通知）。
- \`internal/store\`：自己的 SQLite（任务/勾选/设置/日志）+ 导出 CSV。与 OpenClaw 的库完全分开，只读它的。
- \`internal/openclaw\`：读它的会话库、读它的配置、连它的网关发消息。
- \`internal/app\`：界面与核心之间的胶水（取数据、执行指令、推事件）+ 托盘/图标/开机自启。
- \`internal/cli\`：命令行版（开发期自查用）。
- \`frontend/dist\`：纯原生 JS，无构建工具；调 Go 走 \`window.go.app.App.方法名\`。

## 两条不能混的分界（改并发/列表时最容易搞错）

- \`store.Dispatched()\` = **真在跑的**，引擎算并发用它（\`engine.capWindows\`）。
- \`store.DispatchedSince(开机时刻)\` = **本轮开程序以来派出去过的**（含已完成），只给界面「已派发的」列表用。
- 空列表一律返**空数组**不能返 nil：nil 到界面是 null，取 \`.length\` 就报「Cannot read properties of null」。

## 测试清单（改完至少跑 \`go test -count=1 ./internal/...\`）

${testRows}
`;

const body = head + '\n' + sections.join('\n');
if (process.argv.includes('--check')) {
  console.log(`索引条目 ${total}，文件 ${TARGETS.length}，测试文件 ${TESTS.length}`);
} else {
  fs.writeFileSync(OUT, body, 'utf8');
  console.log(`已写 ${OUT}（条目 ${total}）`);
}
