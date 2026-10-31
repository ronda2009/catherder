# 两个决定性实测：①exe 里 RT_GROUP_ICON 的资源编号（Wails 只认编号 3）②LoadImageW 能否取到 .ico
Add-Type @"
using System;
using System.Text;
using System.Collections.Generic;
using System.Runtime.InteropServices;
public class R {
  [DllImport("kernel32.dll",CharSet=CharSet.Unicode)] static extern IntPtr LoadLibraryExW(string p, IntPtr h, uint f);
  [DllImport("kernel32.dll",CharSet=CharSet.Unicode)] static extern bool EnumResourceNames(IntPtr m, IntPtr t, ENUM_CB cb, IntPtr l);
  [DllImport("kernel32.dll")] static extern bool FreeLibrary(IntPtr m);
  delegate bool ENUM_CB(IntPtr m, IntPtr t, IntPtr n, IntPtr l);
  public static List<string> GroupIconIds(string exe) {
    var res = new List<string>();
    IntPtr m = LoadLibraryExW(exe, IntPtr.Zero, 0x0004); // LOAD_LIBRARY_AS_DATAFILE
    if (m == IntPtr.Zero) { res.Add("LoadLibraryEx 失败 err=" + Marshal.GetLastWin32Error()); return res; }
    IntPtr RT_GROUP_ICON = new IntPtr((IntPtr)(ushort)24);
    EnumResourceNames(m, RT_GROUP_ICON, (mm, tt, nn, ll) => { res.Add("组图标编号=" + (int)(long)nn); return true; }, IntPtr.Zero);
    IntPtr RT_ICON = new IntPtr((IntPtr)(ushort)3);
    EnumResourceNames(m, RT_ICON, (mm, tt, nn, ll) => { res.Add("单图标编号=" + (int)(long)nn); return true; }, IntPtr.Zero);
    FreeLibrary(m);
    return res;
  }
}
"@
Add-Type -Namespace LI -Name I -MemberDefinition '[DllImport("user32.dll",CharSet=CharSet.Unicode,SetLastError=true)] public static extern IntPtr LoadImageW(IntPtr h, string p, int t, int cx, int cy, int fu);'
$exe = 'D:\06.AItools\openclaw\workspace\docs\项目\AI 代理 任务批发工具\app\bin\catherder-gui.exe'
"=== ① exe 里的图标资源编号 ==="
[R]::GroupIconIds($exe) | Select-Object -First 12
"=== ② LoadImageW（uType=1 IMAGE_ICON，之前我误传 0 当游标，测试本身是错的）==="
$tmp = Join-Path $env:TEMP 'catherder-tray.ico'
foreach ($s in @(16, 32, 48)) {
  $h = [LI.I]::LoadImageW([IntPtr]::Zero, $tmp, 1, $s, $s, 0x0010)
  if ($h -eq [IntPtr]::Zero) { "  $s px 失败 err=$([Runtime.InteropServices.Marshal]::GetLastWin32Error())" }
  else { "  $s px 成功" }
}
