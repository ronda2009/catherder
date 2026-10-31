# 导出 exe / 快捷方式当前实际显示的图标（64x64，品红底方便看透明区）
Add-Type -AssemblyName System.Drawing
$out = 'C:\Users\Administrator\.openclaw\media'

function DumpIcon([string]$src, [string]$name) {
  $ic = [System.Drawing.Icon]::ExtractAssociatedIcon($src)
  if (-not $ic) { Write-Output "$name 没抽到图标"; return }
  $bmp = New-Object System.Drawing.Bitmap 64,64
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.Clear([System.Drawing.Color]::Magenta)
  $rect = New-Object System.Drawing.Rectangle(0,0,64,64)
  $g.DrawIcon($ic, $rect)
  $g.Dispose()
  $bmp.Save("$out\$name.png", [System.Drawing.Imaging.ImageFormat]::Png)
  $bmp.Dispose(); $ic.Dispose()
  Write-Output "$name 已导出"
}

DumpIcon 'D:\06.AItools\openclaw\workspace\docs\项目\AI 代理 任务批发工具\app\bin\catherder-gui.exe' 'exe-icon'
DumpIcon 'C:\Users\Administrator\Desktop\墨云(OpenClaw)\工头派工台 CatHerder.lnk' 'lnk-icon'
