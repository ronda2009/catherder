# 列截图层按钮的悬停说明（UIA HelpText = HTML 的 title 属性），验英文界面下提示翻没翻。
# 用法：pwsh -NoProfile -File tools\peek-shot-tooltips.ps1 -Title '测试版'
# 能验出什么：每个标注按钮的悬停说明是什么文字（英文界面下应为英文）。
# 自己的坑：①UIA Name 条件不支持通配符，窗口要自己枚举子窗口用 -like 挑。
#          ②WebView2 网页元素要等 UIA 树建好；截图层没打开时找不到这些按钮。
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

$hits = @()
for ($i = 0; $i -lt 12 -and $hits.Count -lt 3; $i++) {
  $hits = @()
  foreach ($e in $win.FindAll([System.Windows.Automation.TreeScope]::Descendants,
      [System.Windows.Automation.Condition]::TrueCondition)) {
    if ($e.Current.ControlType -eq [System.Windows.Automation.ControlType]::Button) {
      $ht = $e.Current.HelpText
      if ($ht -and $ht.Length -gt 3) { $hits += ('[' + $e.Current.Name + '] ' + $ht) }
    }
  }
  if ($hits.Count -lt 3) { Start-Sleep -Milliseconds 700 }
}
"带悬停说明的按钮 $($hits.Count) 个："
$hits | ForEach-Object { "  $_" }
$zh = $hits | Where-Object { $_ -match '[\u4e00-\u9fff]' }
"其中含中文 $($zh.Count) 个"
