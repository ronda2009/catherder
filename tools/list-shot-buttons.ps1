# 列出截图层里所有按钮的可访问名（UIA Name = 悬停提示文字），用来验英文界面下按钮在不在、提示是不是英文。
# 用法：pwsh -NoProfile -File tools\list-shot-buttons.ps1 -Title '测试版'
# 能验出什么：新工具按钮真在工具条上、悬停提示已翻成英文、有没有残留中文。
# 自己的坑：①UIA 的 Name 条件不支持通配符，窗口要自己枚举子窗口用 -like 挑。
#          ②WebView2 的网页元素要等 UIA 树建好；截图层刚打开时找不到，脚本自己重试。
param([string]$Title = '测试版')
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

$root = [System.Windows.Automation.AutomationElement]::RootElement
$win = $null
for ($i = 0; $i -lt 10 -and -not $win; $i++) {
  foreach ($w in $root.FindAll([System.Windows.Automation.TreeScope]::Children,
      [System.Windows.Automation.Condition]::TrueCondition)) {
    if ($w.Current.Name -like "*$Title*") { $win = $w; break }
  }
  if (-not $win) { Start-Sleep -Milliseconds 600 }
}
if (-not $win) { 'WINDOW_NOTFOUND'; exit 1 }

$names = @()
for ($i = 0; $i -lt 12 -and $names.Count -lt 3; $i++) {
  $names = @()
  foreach ($e in $win.FindAll([System.Windows.Automation.TreeScope]::Descendants,
      [System.Windows.Automation.Condition]::TrueCondition)) {
    if ($e.Current.ControlType -eq [System.Windows.Automation.ControlType]::Button) {
      $n = $e.Current.Name
      if ($n) { $names += $n }
    }
  }
  if ($names.Count -lt 3) { Start-Sleep -Milliseconds 700 }
}
"按钮 $($names.Count) 个："
$names | ForEach-Object { "  [$_]" }
$zh = $names | Where-Object { $_ -match '[\u4e00-\u9fff]' }
if ($zh) { "残留中文 $($zh.Count) 个：" + ($zh -join ' | ') } else { '残留中文 0 个' }
