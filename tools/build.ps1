# 编译两版：命令行版 + 界面版。界面版必须带 -H=windowsgui，否则双击多开一个黑控制台窗。
# 两版都必须带 production 标记，否则 exe 起来只会打印命令行用法、开不了界面。
$env:GOPATH = 'D:\06.AItools\gopath'
$env:GOCACHE = 'D:\06.AItools\gocache'
$env:GOMODCACHE = 'D:\06.AItools\gopath\pkg\mod'
# 版本号：发新版时改项目根下的 VERSION 文件（界面上显示「版本 x.y.z · 本版编译于 …」）。
# 路径写死：下面要 Set-Location 到 app 目录，用相对路径会读不到。
$ver = Get-Content 'D:\06.AItools\openclaw\workspace\docs\项目\AI 代理 任务批发工具\VERSION' -ErrorAction SilentlyContinue

Set-Location 'D:\06.AItools\openclaw\workspace\docs\项目\AI 代理 任务批发工具\app'

if (-not $ver) { $ver = '0.0.0' }
$ver = $ver.Trim()

$stamp = (Get-Date).ToString('yyyy-MM-dd HH:mm')
$ld = "-s -w -X 'catherder/internal/app.BuildStamp=$stamp' -X 'catherder/internal/app.BuildVersion=$ver'"

go build -tags production -trimpath -ldflags $ld -o bin\catherder.exe .
if ($LASTEXITCODE -ne 0) { Write-Output 'CLI 版编译失败'; exit 1 }

go build -tags production,wv2runtime.browser -trimpath -ldflags "-H=windowsgui $ld" -o bin\catherder-gui.exe .
if ($LASTEXITCODE -ne 0) { Write-Output '界面版编译失败'; exit 1 }

Get-ChildItem bin\catherder*.exe | ForEach-Object { '{0} {1} B  {2}' -f $_.Name, $_.Length, $_.LastWriteTime.ToString('HH:mm:ss') }
Write-Output "编译完 版本=$ver stamp=$stamp"
