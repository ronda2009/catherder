# 点界面版窗口里的一个按钮（UI Automation 的 Invoke 模式），用来验证「点一下真生效」。
# 用法：pwsh -NoProfile -File tools\click-gui-button.ps1 -Title '测试版' -Name '取消关机'
# 能验出什么：那个按钮真能点、点了之后后台真收到（配合 peek-test-log.cjs 看日志与时间差）。
# 自己的坑：①UIA 的 Name 条件**不支持通配符**（`*测试版*` 匹配不到），窗口要自己枚举子窗口用 -like 挑。
#          ②WebView2 里的网页元素要等 UIA 树建好才找得到；找不到报 NOTFOUND，不等于按钮不存在。
#          ③托管 API 的 `Invoke()` 不收参数。
param(
  [string]$Title = '测试版',
  [string]$Name = '取消关机'
)
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
if (-not $win) { "WINDOW_NOTFOUND [$Title]"; exit 1 }

$btn = $null
for ($i = 0; $i -lt 10 -and -not $btn; $i++) {
  foreach ($e in $win.FindAll([System.Windows.Automation.TreeScope]::Descendants,
      [System.Windows.Automation.Condition]::TrueCondition)) {
    if ($e.Current.Name -like "*$Name*") { $btn = $e; break }
  }
  if (-not $btn) { Start-Sleep -Milliseconds 600 }
}
if (-not $btn) { "NOTFOUND [$Name]"; exit 1 }

$invoke = [System.Windows.Automation.InvokePattern]$btn.GetCurrentPattern(
  [System.Windows.Automation.InvokePattern]::Pattern)
# 托管 API 的 Invoke() 不收参数（传 Point 会报「找不到重载」）。
$invoke.Invoke()
"CLICKED [$Name] at $(Get-Date -Format 'HH:mm:ss')"
