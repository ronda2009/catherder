# 抓指定进程的窗口截图（用 PrintWindow，窗口被挡住也能拍到），存成 PNG。
param(
  [string]$ProcessName = 'catherder-gui',
  [string]$Title = '',
  [string]$Out = 'D:\06.AItools\openclaw\workspace\tmp\data\gui-shot.png'
)
Add-Type -AssemblyName System.Drawing
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class Win32S {
  [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out Rect lpRect);
  [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr hWnd, IntPtr hdcBlt, uint nFlags);
  [DllImport("user32.dll")] public static extern bool IsIconic(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
  public struct Rect { public int Left, Top, Right, Bottom; }
}
"@

# 必须先声明 DPI 感知：不声明时 GetWindowRect 给的是缩放后的逻辑尺寸，
# 而 PrintWindow 按物理像素画 → 屏幕缩放不是 100% 时右侧会被裁掉一截（实测踩过）。
$null = [Win32S]::SetProcessDPIAware()

$proc = Get-Process -Name $ProcessName -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowHandle -ne 0 }
if ($Title) {
  # 两份同名（正式版/测试版）同时跑时按窗口标题挑，不挑就拍到排在前面那一份。
  $proc = $proc | Where-Object { $_.MainWindowTitle -like "*$Title*" }
}
$proc = $proc | Select-Object -First 1
if (-not $proc) { Write-Output 'NO_WINDOW'; exit 1 }
$h = $proc.MainWindowHandle
[Win32S]::ShowWindow($h, 9) | Out-Null   # SW_RESTORE
[Win32S]::SetForegroundWindow($h) | Out-Null
Start-Sleep -Milliseconds 800

# 刚 SW_RESTORE 完的那几百毫秒里 GetWindowRect 会瞬时报 0x0（实测踩过），所以要重试而不是直接报错退出。
$r = New-Object Win32S+Rect
$w = 0; $hh = 0
for ($i = 0; $i -lt 8; $i++) {
  [void][Win32S]::GetWindowRect($h, [ref]$r)
  $w = $r.Right - $r.Left
  $hh = $r.Bottom - $r.Top
  if ($w -gt 10 -and $hh -gt 10) { break }
  if ($i -eq 2 -or $i -eq 5) { [Win32S]::ShowWindow($h, 9) | Out-Null; [Win32S]::SetForegroundWindow($h) | Out-Null }
  Start-Sleep -Milliseconds 600
}
if ($w -le 10 -or $hh -le 10) { Write-Output "BAD_SIZE $w x $hh"; exit 1 }

$bmp = New-Object System.Drawing.Bitmap($w, $hh)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$hdc = $g.GetHdc()
# PW_RENDERFULLCONTENT = 2：DirectComposition/WebView2 内容也能拍下来
$ok = [Win32S]::PrintWindow($h, $hdc, 2)
$g.ReleaseHdc($hdc)
if (-not $ok) { Write-Output 'PRINTWINDOW_FAILED' }

$dir = Split-Path $Out
if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
$bmp.Save($Out, [System.Drawing.Imaging.ImageFormat]::Png)
$g.Dispose(); $bmp.Dispose()
Write-Output ("SHOT_OK {0}x{1} -> {2}" -f $w, $hh, $Out)

