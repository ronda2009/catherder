// 发布前扫一遍：仓库里跟踪的文件有没有本机痕迹与私人信息。
// 允许出现本机路径的地方只有 tools/*.ps1 与 app/cmd/spike*（开发脚本本来就要写死本机路径）。
// 用法：node tools/peek-privacy.cjs
const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const files = execSync('git ls-files', { cwd: ROOT, encoding: 'utf8' }).split('\n').filter(Boolean);

const PATTERNS = [
  ['账号ID(ou_…)', /\bou_[0-9a-f]{16,}\b/],
  ['群ID(oc_…)', /\noc_[0-9a-f]{16,}\b/],
  ['GitHub token(gho_…)', /\bgho_[A-Za-z0-9]{6,}\b/],
  ['用户名(Administrator)', /\bAdministrator\b/],
  ['本机路径 C:\\Users', /C:\\Users/],
  ['本机路径 D:\\06', /D:\\06/],
  ['窗口名(墨云/绘图员/开发员/升级助手/按钮开发组/飞Q)', /(墨云|绘图员|开发员|升级助手|按钮开发组|飞Q)/],
];

// 开发脚本本来就要写死本机路径与真实窗口名，不算泄露
const ALLOW = /^(tools\/|app\/cmd\/spike\d?\/)/;
const hits = [];
for (const rel of files) {
  const p = path.join(ROOT, rel.replace(/\//g, path.sep));
  if (!fs.existsSync(p) || fs.statSync(p).isDirectory()) continue;
  let body;
  try { body = fs.readFileSync(p, 'utf8'); } catch { continue; }
  const lines = body.split('\n');
  for (const [name, re] of PATTERNS) {
    for (let i = 0; i < lines.length; i++) {
      if (re.test(lines[i])) {
        hits.push({ file: rel, name, line: i + 1, allowed: ALLOW.test(rel), text: lines[i].trim().slice(0, 90) });
        break; // 每文件每类只报首处
      }
    }
  }
}

const bad = hits.filter((h) => !h.allowed);
const ok = hits.filter((h) => h.allowed);
console.log(`跟踪文件 ${files.length} 个。允许区（tools/ 开发脚本、app/cmd/spike* 探针）命中 ${ok.length} 处，其余命中 ${bad.length} 处。`);
if (bad.length) {
  console.log('\n【发布前要处理的】');
  for (const h of bad) console.log(`  ${h.file} L${h.line}  ${h.name}\n      ${h.text}`);
} else {
  console.log('\n【其余位置干净】README/CHANGELOG/LICENSE/代码里没有本机路径、账号ID、群ID、token、真实窗口名。');
}
console.log('\n【允许区命中明细（开发脚本写死本机路径，属正常）】');
const byFile = {};
for (const h of ok) (byFile[h.file] = byFile[h.file] || []).push(h.name);
for (const f of Object.keys(byFile)) console.log(`  ${f}: ${byFile[f].join('、')}`);
