# 查窗口现在到底有没有图标（WM_GETICON 实时读，不靠我写的代码猜）
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class GI {
  [DllImport("user32.dll")] public static extern IntPtr SendMessage(IntPtr h, int msg, IntPtr w, IntPtr l);
  [DllImport("user32.dll")] public static extern bool GetIconInfo(IntPtr h, out II info);
  [StructLayout(LayoutKind.Sequential)] public struct II { public int flags; public int w; public int h; public IntPtr xhot; public IntPtr yhot; public IntPtr hbmIcon; }
}
"@
$p = Get-Process catherder-gui -ErrorAction SilentlyContinue | Select-Object -First 1
if (-not $p) { 'no-process'; exit }
$h = $p.MainWindowHandle
"hwnd=$h"
foreach ($case in @(@('小图位', 0), @('大图位', 1))) {
  $name = $case[0]; $which = $case[1]
  $ico = [GI]::SendMessage($h, 0x007F, [IntPtr]$which, [IntPtr]::Zero)  # WM_GETICON
  if ($ico -eq [IntPtr]::Zero) { $ico = [GI]::SendMessage($h, 0x0033, [IntPtr]$which, [IntPtr]::Zero) } # WM_GETSMALLICON fallback
  if ($ico -eq [IntPtr]::Zero) { "{0}: 没挂上（0）" -f $name }
  else {
    $ii = New-Object GI+II
    if ([GI]::GetIconInfo($ico, [ref]$ii)) { "{0}: 有图标 {1}x{2}" -f $name, $ii.w, $ii.h }
    else { "{0}: 句柄={1} 但读不到尺寸" -f $name, $ico }
  }
}
