// 托盘探针（只读 + 模拟鼠标，不改数据）：
//
// find 判「只收消息的托盘窗」在不在（排除 PowerShell P/Invoke 的假阴性）
// state 看主窗（wailsWindow）可见性
// click 给托盘图标发一次「左键抬起」→ 应叫回主界面
// close 给主窗发 WM_CLOSE（模拟点右上角叉）→ 应藏进托盘不退出
package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	user32  = syscall.NewLazyDLL("user32.dll")
	gdi32   = syscall.NewLazyDLL("gdi32.dll")
	findEx  = user32.NewProc("FindWindowExW")
	find    = user32.NewProc("FindWindowW")
	post    = user32.NewProc("PostMessageW")
	isVis   = user32.NewProc("IsWindowVisible")
	getCls  = user32.NewProc("GetClassNameW")
	enumWin = user32.NewProc("EnumWindows")
	getPID  = user32.NewProc("GetWindowThreadProcessId")
)

const (
	hwndMessage = uintptr(^uint(0) - 2) // (HWND)-3
	wmClose     = 0x0010
	wmTrayCb    = 0x0401
	wmLButtonUp = 0x0202
	trayClass   = "CatHerderTrayWindow"
	wailsClass  = "wailsWindow"
)

func ptr(s string) uintptr {
	p, _ := syscall.UTF16PtrFromString(s)
	return uintptr(unsafe.Pointer(p))
}

func trayWnd() uintptr {
	r, _, _ := findEx.Call(hwndMessage, 0, ptr(trayClass), 0)
	if r == 0 {
		r, _, _ = find.Call(ptr(trayClass), 0)
	}
	return r
}

// mainWnd 按类名 + 进程 id 找界面版那个 Wails 主窗（避开机器上别的 wails 程序）。
func mainWnd() uintptr {
	var found uintptr
	cb := syscall.NewCallback(func(h uintptr, l uintptr) uintptr {
		var p uint32
		getPID.Call(h, uintptr(unsafe.Pointer(&p)))
		if p != targetPID {
			return 1
		}
		buf := make([]uint16, 64)
		getCls.Call(h, uintptr(unsafe.Pointer(&buf[0])), 64)
		if syscall.UTF16ToString(buf) == wailsClass {
			found = h
			return 0
		}
		return 1
	})
	enumWin.Call(cb, 0)
	return found
}

var targetPID uint32

func visible(h uintptr) bool {
	r, _, _ := isVis.Call(h)
	return r != 0
}

func main() {
	cmd := "find"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	// 界面版的进程号由外面传进来（PowerShell 用 Get-Process 取最可靠）。
	if len(os.Args) > 2 && os.Args[2] != "0" {
		var n int
		fmt.Sscanf(os.Args[2], "%d", &n)
		targetPID = uint32(n)
	}
	if targetPID == 0 && cmd != "icon" && cmd != "find" {
		fmt.Println("没传界面版的进程号：trayprobe <命令> <pid>")
		return
	}

	switch cmd {
	case "find":
		t := trayWnd()
		fmt.Printf("托盘窗 hwnd=%d（0=没建成）\n", t)
		r3, _, _ := findEx.Call(hwndMessage, 0, ptr("wails-app-catherder-paigongtai-sic"), 0)
		fmt.Printf("对照·单实例事件窗 hwnd=%d\n", r3)

	case "state":
		t := trayWnd()
		m := mainWnd()
		fmt.Printf("托盘窗 hwnd=%d  主窗 hwnd=%d 主窗可见=%v\n", t, m, visible(m))

	case "click":
		t := trayWnd()
		if t == 0 {
			fmt.Println("托盘窗不在，没法模拟点击")
			return
		}
		r, _, err := post.Call(t, wmTrayCb, 0, wmLButtonUp)
		fmt.Printf("已给托盘图标发左键（PostMessage 成功=%v err=%v）\n", r != 0, err)

	case "icon":
		// 直接验 Wails 取图标那条路：LoadIcon(模块, IDI_APPLICATION)。
		// PNG 帧的 .ico 会被它判失败 → 任务栏/窗口图标变「出错」样。
		// 用法: trayprobe icon <exe路径>（不给路径就查探针自己）
		kernel32 := syscall.NewLazyDLL("kernel32.dll")
		getMod := kernel32.NewProc("GetModuleHandleW")
		loadLib := kernel32.NewProc("LoadLibraryExW")
		loadIcon := user32.NewProc("LoadIconW")
		loadImg := user32.NewProc("LoadImageW")
		enumRes := kernel32.NewProc("EnumResourceNamesW")
		const idiApplication = 32512
		var hinst uintptr
		if len(os.Args) > 3 {
			p, _ := syscall.UTF16PtrFromString(os.Args[3])
			hinst, _, _ = loadLib.Call(uintptr(unsafe.Pointer(p)), 0, 0x00000040) // LOAD_LIBRARY_AS_DATAFILE
			fmt.Printf("按数据文件载入 %s: 模块=%d\n", os.Args[3], hinst)
		} else {
			hinst, _, _ = getMod.Call(0)
		}
		if hinst == 0 {
			fmt.Println("载不进那个 exe")
			return
		}
		h, _, err := loadIcon.Call(hinst, uintptr(idiApplication))
		fmt.Printf("LoadIcon(IDI_APPLICATION): 句柄=%d 成功=%v err=%v\n", h, h != 0, err)

		// 逐个试常见组名（A / 1 / 32512）：rsrc 生成的组名可能是「1」而不是「A」
		findRes := kernel32.NewProc("FindResourceW")
		loadRes := kernel32.NewProc("LoadResource")
		for _, nm := range []string{"A", "1"} {
			p, _ := syscall.UTF16PtrFromString(nm)
			hr, _, e := findRes.Call(hinst, uintptr(unsafe.Pointer(p)), 14) // RT_GROUP_ICON=14
			fmt.Printf("FindResource(组图标 名=%s): 句柄=%d err=%v\n", nm, hr, e)
			if hr != 0 {
				hr3, _, e3 := findRes.Call(hinst, uintptr(unsafe.Pointer(p)), 3) // RT_ICON
				fmt.Printf("  同名单个图标: 句柄=%d err=%v\n", hr3, e3)
				if hr3 != 0 {
					lv, _, _ := loadRes.Call(hinst, hr3)
					fmt.Printf("  LoadResource: %d\n", lv)
				}
			}
		}
		// 按数字 id 找组图标
		for id := uintptr(1); id <= 3; id++ {
			hr, _, _ := findRes.Call(hinst, id, 14)
			fmt.Printf("FindResource(组图标 id=%d): 句柄=%d\n", id, hr)
		}
		// 用 LoadImage 按名字加载组图标（IMAGE_ICON=1，LR_SHARED=0x0040，LR_LOADTRANSPARENT 不用）
		for _, nm := range []string{"A", "1"} {
			p, _ := syscall.UTF16PtrFromString(nm)
			hx, _, ex := loadImg.Call(hinst, uintptr(unsafe.Pointer(p)), 1, 0, 0, 0x0040)
			fmt.Printf("LoadImage(组图标 名=%s): 句柄=%d 成功=%v err=%v\n", nm, hx, hx != 0, ex)
		}

		// IMAGE_ICON=1，LR_SHARED=0x0040；按小图标尺寸再试一次
		getSys := user32.NewProc("GetSystemMetrics")
		cx, _, _ := getSys.Call(49) // SM_CXSMICON
		cy, _, _ := getSys.Call(50) // SM_CYSMICON
		h2, _, err2 := loadImg.Call(hinst, uintptr(idiApplication), 1, cx, cy, 0x0040)
		fmt.Printf("LoadImage(小图标 %dx%d): 句柄=%d 成功=%v err=%v\n", cx, cy, h2, h2 != 0, err2)

		// 列出 exe 里的图标资源（RT_ICON=3，RT_GROUP_ICON=14）
		for _, rt := range []struct {
			name string
			id   uintptr
		}{
			{"RT_GROUP_ICON", 14},
			{"RT_ICON", 3},
		} {
			var names []string
			cb := syscall.NewCallback(func(hm uintptr, rt uintptr, name *uint16, data uintptr) uintptr {
				if name != nil {
					names = append(names, fmt.Sprintf("%d", uintptr(unsafe.Pointer(name))))
				} else {
					names = append(names, "?")
				}
				return 1
			})
			ok, _, _ := enumRes.Call(hinst, rt.id, uintptr(cb), 0)
			fmt.Printf("%s 枚举成功=%v 个数=%d\n", rt.name, ok != 0, len(names))
		}

	case "seticon":
		// 手动给主窗挂图标（验「我们那段挂图标的代码到底能不能成」）：
		// trayprobe seticon <pid> [图标文件路径]
		m := mainWnd()
		if m == 0 {
			fmt.Println("主窗没找到")
			return
		}
		ico := os.Getenv("TEMP") + "\\catherder-tray.ico"
		if len(os.Args) > 3 {
			ico = os.Args[3]
		}
		getSys := user32.NewProc("GetSystemMetrics")
		sx, _, _ := getSys.Call(49)
		sy, _, _ := getSys.Call(50)
		bigX, _, _ := getSys.Call(11)
		bigY, _, _ := getSys.Call(12)
		path, _ := syscall.UTF16PtrFromString(ico)
		load := user32.NewProc("LoadImageW")
		send := user32.NewProc("SendMessageW")
		hs, _, e1 := load.Call(0, uintptr(unsafe.Pointer(path)), 1, sx, sy, 0x10)
		fmt.Printf("小图标 %dx%d 载入: 句柄=%d err=%v\n", sx, sy, hs, e1)
		if hs != 0 {
			send.Call(m, 0x0028 /*WM_SETICON*/, 0, hs)
		}
		hb, _, e2 := load.Call(0, uintptr(unsafe.Pointer(path)), 1, bigX, bigY, 0x10)
		fmt.Printf("大图标 %dx%d 载入: 句柄=%d err=%v\n", bigX, bigY, hb, e2)
		if hb != 0 {
			send.Call(m, 0x0028, 1, hb)
		}
		fmt.Println("已发 WM_SETICON")

	case "geticon":
		// 问主窗要它现在挂的图标（WM_GETICON）：句柄 0 ＝ 没挂我们自己的
		// 那任务栏/标题栏显示的就是系统通用图标（用户看到的「像出错了那种」）。
		m := mainWnd()
		if m == 0 {
			fmt.Println("主窗没找到")
			return
		}
		sendMsg := user32.NewProc("SendMessageW")
		getII := user32.NewProc("GetIconInfo")
		getObject := gdi32.NewProc("GetObjectW")
		type iconInfo struct {
			fIcon    int32
			xHot     int32
			yHot     int32
			hbmMask  uintptr
			hbmColor uintptr
		}
		type bitmap struct {
			w, h       int32
			planesBits int16
			bytes      int32
			bits       uintptr
		}
		dump := func(kind string, idx uintptr) {
			h, _, _ := sendMsg.Call(m, 0x007F /*WM_GETICON*/, idx)
			fmt.Printf("%s 图标句柄=%d（0＝没挂自己的图标）\n", kind, h)
			if h == 0 {
				return
			}
			var ii iconInfo
			getII.Call(h, uintptr(unsafe.Pointer(&ii)))
			if ii.hbmColor != 0 {
				var bm bitmap
				getObject.Call(ii.hbmColor, 76, uintptr(unsafe.Pointer(&bm)))
				fmt.Printf("   彩色图尺寸 = %dx%d\n", bm.w, bm.h)
			}
		}
		dump("小图（任务栏按钮+标题栏）", 0)
		dump("大图（Alt+Tab）", 1)

	case "close":
		m := mainWnd()
		if m == 0 {
			fmt.Println("主窗没找到")
			return
		}
		r, _, err := post.Call(m, wmClose, 0, 0)
		fmt.Printf("已给主窗发 WM_CLOSE（成功=%v err=%v）\n", r != 0, err)

	default:
		fmt.Println("用法: find | state | click | close | icon | geticon | seticon")
	}
}
