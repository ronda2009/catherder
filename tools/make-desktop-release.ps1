# 把当前编译好的界面版复制到桌面「墨云(OpenClaw)」当正式版用，并建快捷方式。
# 关键：给这份独立的锁号(--id)、独立的名字(--label)、独立的库(--data)，
# 才能和我 workspace 里开着测试的那份同时跑、互不干扰。
$ErrorActionPreference = 'Stop'
$proj = 'D:\06.AItools\openclaw\workspace\docs\项目\AI 代理 任务批发工具'
$dst  = 'C:\Users\Administrator\Desktop\墨云(OpenClaw)\工头派工台 正式版'

New-Item -ItemType Directory -Force -Path $dst | Out-Null
Copy-Item ($proj + '\app\bin\catherder-gui.exe') ($dst + '\catherder-gui.exe') -Force

$exe  = $dst + '\catherder-gui.exe'
$db   = $dst + '\catherder.sqlite'
$icon = $proj + '\app\assets\tray.ico'
$label = '工头派工台 CatHerder（正式版）'
$args  = '-id catherder-formal -label "' + $label + '" -data "' + $db + '"'

$sh = New-Object -ComObject WScript.Shell
$lnkPath = 'C:\Users\Administrator\Desktop\墨云(OpenClaw)\工头派工台 正式版.lnk'
$lnk = $sh.CreateShortcut($lnkPath)
$lnk.TargetPath       = $exe
$lnk.Arguments        = $args
$lnk.WorkingDirectory = $dst
$lnk.IconLocation     = $icon + ',0'
$lnk.Description      = '工头派工台 正式版（独立数据，可与测试版同时运行）'
$lnk.Save()

$g = Get-Item $exe
'已复制  ' + $exe + '  ' + $g.Length + 'B  ' + $g.LastWriteTime.ToString('HH:mm:ss')
'快捷方式 ' + $lnkPath
'参数    ' + $args
'库位置  ' + $db
