# 开机自启（注册表 HKCU\...\Run\CatHerder）指向哪一份派工台。
# 用法：
#   pwsh -NoProfile -File tools\set-autostart.ps1            # 指到桌面正式版（带正式版的锁号/标题/库）
#   pwsh -NoProfile -File tools\set-autostart.ps1 -Show      # 只看现在指哪份，不改
#   pwsh -NoProfile -File tools\set-autostart.ps1 -RestoreFile <备份文件>   # 照备份文件回滚
# 每次改之前把旧值存到 tools\autostart-旧值-<日期时间>.txt，回滚就用 -RestoreFile。
param(
  [switch]$Show,
  [string]$RestoreFile = ''
)
$ErrorActionPreference = 'Stop'
$run = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$name = 'CatHerder'
$proj = 'D:\06.AItools\openclaw\workspace\docs\项目\AI 代理 任务批发工具'
$dst  = 'C:\Users\Administrator\Desktop\墨云(OpenClaw)\工头派工台 正式版'
$exe  = Join-Path $dst 'catherder-gui.exe'
$db   = Join-Path $dst 'catherder.sqlite'
$label = '工头派工台 CatHerder（正式版）'
# 锁号/标题/库三项都要和桌面快捷方式一致：开机那份和手动双击的才是同一个程序（不会开出两份）。
$new = '"' + $exe + '" -id catherder-formal -label "' + $label + '" -data "' + $db + '"'

$old = (Get-ItemProperty -Path $run -Name $name -ErrorAction SilentlyContinue).$name
"现值: " + $(if ($old) { $old } else { '(没有这一项)' })
if ($Show) { return }

if ($RestoreFile) {
  if (-not (Test-Path $RestoreFile)) { throw "备份文件不存在: $RestoreFile" }
  $new = (Get-Content -Path $RestoreFile -Raw -Encoding utf8).Trim()
  "回滚成: $new"
}

$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$backup = Join-Path $proj ("tools\autostart-旧值-" + $stamp + ".txt")
Set-Content -Path $backup -Value $(if ($old) { $old } else { '' }) -Encoding utf8 -NoNewline
"旧值已备份: $backup"

if (-not (Test-Path $exe)) { throw "正式版 exe 不存在: $exe" }
New-ItemProperty -Path $run -Name $name -Value $new -PropertyType String -Force | Out-Null
Set-ItemProperty -Path $run -Name $name -Value $new

$now = (Get-ItemProperty -Path $run -Name $name).$name
"新值: $now"
if ($now -ne $new) { throw "写完复核不一致" }
'OK 开机自启已指向正式版'
