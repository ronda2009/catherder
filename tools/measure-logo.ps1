# 量当前界面上：小猫头黄帽的上沿/下沿、以及「工头派工台」文字的上沿，算出该放大到多少
Add-Type -AssemblyName System.Drawing
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class Q1 {
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out R r);
  [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr h, IntPtr dc, uint f);
  [StructLayout(LayoutKind.Sequential)] public struct R { public int l, t, r, b; }
}
"@
$p = Get-Process catherder-gui -ErrorAction SilentlyContinue | Select-Object -First 1
if (-not $p) { 'no-process'; exit }
$h = $p.MainWindowHandle
$rc = New-Object Q1+R; $null = [Q1]::GetWindowRect($h, [ref]$rc)
$W = $rc.r - $rc.l; $Hh = $rc.b - $rc.t
$bmp = New-Object System.Drawing.Bitmap $W, $Hh
$g = [System.Drawing.Graphics]::FromImage($bmp)
$null = [Q1]::PrintWindow($h, $g.GetHdc(), 2)
$g.Dispose()

function IsYellow($c) { return ($c.R -gt 200 -and $c.G -gt 130 -and $c.B -lt 130) }
function IsBright($c) { return ($c.R -gt 190 -and $c.G -gt 190 -and $c.B -gt 190) }

# 1) 小猫头黄帽：在左上角 logo 区域扫（x 30..95, y 55..150）
$capTop = -1; $capBot = -1; $logoL = -1; $logoR = -1
for ($y = 55; $y -lt 150; $y++) {
  for ($x = 25; $x -lt 100; $x++) {
    $c = $bmp.GetPixel($x, $y)
    if (IsYellow $c) {
      if ($capTop -lt 0) { $capTop = $y }
      $capBot = $y
      if ($logoL -lt 0 -or $x -lt $logoL) { $logoL = $x }
      if ($x -gt $logoR) { $logoR = $x }
    }
  }
}
# 2) 「工头派工台」大字上沿：在 logo 右边扫（x 100..320, y 55..120）
$txtTop = -1; $txtBot = -1
for ($y = 55; $y -lt 125; $y++) {
  $cnt = 0
  for ($x = 100; $x -lt 330; $x++) { if (IsBright ($bmp.GetPixel($x, $y))) { $cnt++ } }
  if ($cnt -ge 6) { if ($txtTop -lt 0) { $txtTop = $y }; $txtBot = $y }
}
$bmp.Dispose()

"黄帽上沿 y=$capTop  下沿 y=$capBot  宽=($logoR-$logoL+1)"
"文字上沿 y=$txtTop  下沿 y=$txtBot"
if ($capTop -ge 0 -and $capBot -ge 0 -and $txtTop -ge 0) {
  $curImg = $capBot - $capTop + 1
  "黄帽本身高=$curImg（图内黄帽占的高度）"
  "要让黄帽上沿=文字上沿，需要再往上抬 $($capTop - $txtTop) 像素"
  "按老板说的「下沿不动、上沿抬到与文字持平或略高」：图片高度应增加约 $($capTop - $txtTop) 像素"
}
