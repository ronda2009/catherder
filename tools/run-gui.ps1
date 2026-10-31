# 启动界面版（测试用）并回标题（验证起没起来）。加 -Debug 顺带跑界面自检。
# 只杀「测试版」自己那份（按命令行里的 -id catherder-dev 认），
# 不碰老板挂在桌面「工头派工台 正式版」那份——那份用 -id catherder-formal。
param([switch]$Debug)
Set-Location 'D:\06.AItools\openclaw\workspace\docs\项目\AI 代理 任务批发工具\app\bin'

Get-CimInstance Win32_Process -Filter "Name='catherder-gui.exe'" -ErrorAction SilentlyContinue |
  Where-Object { $_.CommandLine -match 'catherder-dev' } |
  ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
Start-Sleep -Seconds 2

$exe = (Get-Location).Path + '\catherder-gui.exe'
# 测试版用自己的锁号与名字：与桌面正式版并存、互不合并、各管各的库。
$cmd = '"' + $exe + '" -id catherder-dev -label "工头派工台 CatHerder（测试版）"'
if ($Debug) { $cmd = $cmd + ' -tab debug' }

# 必须用 Invoke-CimMethod 起进程：Start-Process 起的 GUI 进程有时拿不到窗口（实测）。
$r = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{ CommandLine = $cmd; CurrentDirectory = (Get-Location).Path }
Start-Sleep -Seconds 14
$q = Get-Process -Id $r.ProcessId -ErrorAction SilentlyContinue
if ($q) { 'pid={0} 标题=[{1}] 内存={2}MB' -f $q.Id, $q.MainWindowTitle, [Math]::Round($q.WorkingSet64 / 1MB, 1) }
else { '进程不在（启动即退）' }

# 顺带报现在一共跑着几份、分别是谁（两份并存时老板要能分清）。
Get-CimInstance Win32_Process -Filter "Name='catherder-gui.exe'" -ErrorAction SilentlyContinue |
  ForEach-Object {
    $tag = if ($_.CommandLine -match 'catherder-formal') { '正式版' } elseif ($_.CommandLine -match 'catherder-dev') { '测试版' } else { '默认' }
    '在跑 pid=' + $_.ProcessId + ' [' + $tag + ']'
  }
