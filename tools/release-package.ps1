# 打 GitHub Release 包：编译 → 组目录 → 压缩 → 报体积与 SHA256。
# 版本号取项目根 VERSION；产物名 catherder-<版本>-windows-<架构>.zip。
# 用法：pwsh -File tools\release-package.ps1 [-SkipBuild]
param([switch]$SkipBuild)
$ErrorActionPreference = 'Stop'
$proj = 'D:\06.AItools\openclaw\workspace\docs\项目\AI 代理 任务批发工具'
$ver = (Get-Content ($proj + '\VERSION') -ErrorAction SilentlyContinue).Trim()
if (-not $ver) { Write-Output 'VERSION 读不到'; exit 1 }
$arch = (Get-CimInstance Win32_OperatingSystem).OSArchitecture -replace '64-bit', 'amd64' -replace ' ', ''
if ($arch -ne 'amd64') { $arch = 'amd64' }

if (-not $SkipBuild) { pwsh -File ($proj + '\tools\build.ps1') | Out-Null }

$stage = Join-Path $env:TEMP ("catherder-rel-" + $ver)
$bin = Join-Path $stage "catherder-$ver-windows-$arch"
Remove-Item $stage -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $bin | Out-Null

Copy-Item ($proj + '\app\bin\catherder-gui.exe') ($bin + '\catherder-gui.exe')
Copy-Item ($proj + '\app\bin\catherder.exe')     ($bin + '\catherder.exe')
foreach ($f in @('README.md', 'README.zh-CN.md', 'CHANGELOG.md', 'LICENSE')) { Copy-Item ($proj + '\' + $f) ($bin + '\' + $f) }
Set-Content -Path ($bin + '\README-安装说明.txt') -Encoding UTF8 -Value @(
  "工头派工台 CatHerder $ver（Windows 免安装版）",
  '',
  '1. 解压到任意文件夹。',
  '2. 双击 catherder-gui.exe 运行；需本机已运行 OpenClaw 网关（默认端口 18789，自动读取）。',
  '3. 数据存在 %APPDATA%\catherder\catherder.sqlite；多实例用 -data <路径> 指定独立库。',
  '4. 命令行版：catherder.exe queue / status / log / export / tick。',
  '5. 界面中英文可切（设置页）；详细说明见 README。'
)

$zip = Join-Path $env:TEMP ("catherder-$ver-windows-$arch.zip")
Remove-Item $zip -Force -ErrorAction SilentlyContinue
Compress-Archive -Path $bin -DestinationPath $zip

$h = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
$size = (Get-Item $zip).Length
Write-Output ("打包完 {0}  {1} B" -f $zip, $size)
Write-Output ("SHA256 {0}" -f $h)
Write-Output ("gh 发布命令：gh release create v{0} ""{1}"" --title ""v{0}"" --notes-file <发布说明文件>" -f $ver, $zip)
