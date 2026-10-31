// 派工台（CatHerder）唯一入口：带子命令走命令行，否则开界面。
// 两种模式共用同一套 internal/*，所以命令行验证过的逻辑在界面里就是同一份。
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	gui "catherder/internal/app"
	"catherder/internal/cli"
	"catherder/internal/engine"
	"catherder/internal/openclaw"
	"catherder/internal/store"
)

// 前端资源嵌进 exe（纯原生 HTML/CSS/JS，不需要 npm 构建）。
//
//go:embed all:frontend/dist
var rawAssets embed.FS

// 右下角任务栏图标也嵌进 exe：单文件发布，不依赖旁边有 .ico。
//
//go:embed assets/tray.ico
var trayIcon []byte

var assets fs.FS = func() fs.FS {
	f, err := fs.Sub(rawAssets, "frontend/dist")
	if err != nil {
		panic(err)
	}
	return f
}()

// Windows 控制台默认 GBK 码页，Go 写的 UTF-8 中文会变乱码；命令行模式要切过来。
func init() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	kernel32.NewProc("SetConsoleOutputCP").Call(65001)
}

// looksLikeSubcommand 判断第一个参数像不像「想跑子命令」：纯字母数字连字号、不是开关、
// 不是路径、不是文件名。只有这种才拦下来报「没这个子命令」，否则当普通参数走界面。
func looksLikeSubcommand(word string) bool {
	if word == "" || strings.HasPrefix(word, "-") {
		return false
	}
	if strings.ContainsAny(word, `\/.")`) || strings.Contains(word, ":") {
		return false // 路径、带扩展名的文件、窗口键之类
	}
	for _, r := range word {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func main() {
	argv := os.Args[1:]
	if len(argv) > 0 && cli.IsCommand(argv[0]) {
		cli.Run(argv)
		return
	}
	// 参数不是已知子命令也不是开关（打错了、或新加命令忘了进白名单）：给一句人话用法。
	// 不拦的话会掉进界面模式：命令行版没带 production 标记时 wails.Run 直接崩，
	// 甩一堆 Go 栈给用户看。
	if len(argv) > 0 && looksLikeSubcommand(argv[0]) {
		fmt.Printf("没这个子命令：%s\n\n", argv[0])
		cli.Usage()
		os.Exit(2)
	}

	// 界面模式也认这三个参数，方便测试时指定别的库文件。
	fsFlags := flag.NewFlagSet("catherder", flag.ContinueOnError)
	configPath := fsFlags.String("config", openclaw.DefaultConfigPath(), "openclaw.json 路径")
	dbPath := fsFlags.String("db", openclaw.DefaultDBPath("main"), "OpenClaw 会话库（只读）")
	dataPath := fsFlags.String("data", store.DefaultPath(), "派工台自己的库")
	startTab := fsFlags.String("tab", "", "启动时先打开哪个页：dispatch/windows/log/settings（调试用）")
	height := fsFlags.Int("height", 780, "窗口高度（调试用）")
	// 同时跑两份（老板挂后台用 + 我开着测试）：单实例锁原来写死一个号，
	// 第二份会被当成「重复双击」并进第一个。--id 给每份自己的锁号，--data 给每份自己的库。
	instID := fsFlags.String("id", "catherder-paigongtai", "单实例锁号：两份要同时跑就给不同的号")
	label := fsFlags.String("label", "工头派工台 CatHerder", "窗口标题与右下角图标悬停显示的名字（两份并存时用来区分）")
	if err := fsFlags.Parse(argv); err != nil {
		fmt.Println("参数不对:", err)
		os.Exit(2)
	}

	cfg, err := openclaw.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("读不到 OpenClaw 配置：%v\n（界面里仍可手填网关端口与密钥）\n", err)
		cfg = &openclaw.Config{}
	}
	gui.SetTrayTitle(*label)

	st, err := store.Open(*dataPath)
	if err != nil {
		fmt.Printf("打不开派工台自己的库：%v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	eng := engine.New(cfg, st, *dbPath)
	app := gui.New(gui.Deps{
		Cfg: cfg, Store: st, Engine: eng,
		IntervalSec: eng.SettingInterval(),
		StartTab:    *startTab,
		TrayIcon:    trayIcon,
		// 截图放自己的库旁边（跟着 --data 走，换机器/换目录不用改代码）。
		ImageDir: filepath.Join(filepath.Dir(*dataPath), "截图"),
	})

	// 没带 production 标记编出来的 exe 开界面会在 wails.Run 里直接崩（：
	// 命令行版误用无标记编译，参数又没被认成子命令，就掉到这里甲一堆 Go 栈）。
	if !gui.BuiltProduction {
		fmt.Println("这个 exe 编译时没带 production 标记，开不了界面。")
		fmt.Println("命令行用法如下：")
		fmt.Println()
		cli.Usage()
		os.Exit(2)
	}

	if err := wails.Run(&options.App{
		Title:     *label,
		Width:     1180,
		Height:    *height,
		MinWidth:  900,
		MinHeight: 620,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// 窗口底色定黑：全屏/退全屏那一瞬 WebView 重绘时露出的是底色，
		// 不定的话默认是白 → 老板看到的「闪一下」。
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 1},
		// 文件能直接拖进窗口（拖到输入区那块）：和网页端一样方便。
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
		// 标题栏强制深色。
		// Theme=Dark 走沉浸式深色（Win10 1809+ 支持）；CustomTheme 里的标题栏颜色
		// 只有 Win11 才生效（DwmwaCaptionColor 是 Win11 属性），留着让 Win11 上更贴界面底色。
		// 失焦（inactive）那组也必须填，否则 Win11 上窗口一失焦标题栏会变黑。
		Windows: &windows.Options{
			Theme: windows.Dark,
			CustomTheme: &windows.ThemeSettings{
				DarkModeTitleBar:           windows.RGB(28, 32, 39),
				DarkModeTitleBarInactive:   windows.RGB(35, 40, 49),
				DarkModeTitleText:          windows.RGB(230, 233, 239),
				DarkModeTitleTextInactive:  windows.RGB(152, 162, 179),
				DarkModeBorder:             windows.RGB(46, 52, 64),
				DarkModeBorderInactive:     windows.RGB(40, 45, 55),
				LightModeTitleBar:          windows.RGB(28, 32, 39),
				LightModeTitleBarInactive:  windows.RGB(35, 40, 49),
				LightModeTitleText:         windows.RGB(230, 233, 239),
				LightModeTitleTextInactive: windows.RGB(152, 162, 179),
				LightModeBorder:            windows.RGB(46, 52, 64),
				LightModeBorderInactive:    windows.RGB(40, 45, 55),
			},
		},
		OnStartup:  app.Startup,
		OnShutdown: app.Shutdown,
		// 点右上角叉：收到右下角任务栏继续干活。
		// 托盘没起来或设置里关了收到托盘 → 不拦，照常退出（免得窗口被藏没了又没地方叫回）。
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			if app.ShouldHideOnClose() {
				app.HideToTray()
				return true
			}
			return false
		},
		// 常驻后重复双击快捷方式：不开第二个，把已收起来的那个叫回桌面。
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: *instID,
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				app.ShowMainWindow()
			},
		},
		Bind: []interface{}{app},
	}); err != nil {
		fmt.Println("界面退出:", err)
	}
}
