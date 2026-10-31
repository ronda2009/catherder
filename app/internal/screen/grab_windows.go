//go:build windows

// 屏幕抓取（Windows）—— 独立小包，界面版与命令行版共用
// 这里只做「把整屏抓成 PNG」这一件事；选区与标注交给界面里的画布做
// （复用现成 WebView；Wails v2 不支持第二个窗口，再写一套 Win32 绘图窗是往上堆东西）。
// 抓屏走 GDI：GetDC(NULL) 拿整屏设备上下文 → CreateDIBSection 直接要到一块
// 可写的 32bpp 像素内存 → BitBlt 把屏幕拷进去 → 直接读那块内存编成 PNG。
// CreateDIBSection 少一步、拿到的是自己的内存，不绕。
// 本程序 manifest 声明了 PerMonitorV2，GetSystemMetrics 给的是物理像素，抓出来不糊。
package screen

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"syscall"
	"unsafe"
)

var (
	user32            = syscall.NewLazyDLL("user32.dll")
	gdi32             = syscall.NewLazyDLL("gdi32.dll")
	procGetDC         = user32.NewProc("GetDC")
	procReleaseDC     = user32.NewProc("ReleaseDC")
	procGetSysMetrics = user32.NewProc("GetSystemMetrics")
	procCreateCompDC  = gdi32.NewProc("CreateCompatibleDC")
	procSelectObject  = gdi32.NewProc("SelectObject")
	procBitBlt        = gdi32.NewProc("BitBlt")
	procCreateDIBSect = gdi32.NewProc("CreateDIBSection")
	procGdiFlush      = gdi32.NewProc("GdiFlush")
	procDeleteObject  = gdi32.NewProc("DeleteObject")
	procDeleteDC      = gdi32.NewProc("DeleteDC")
)

const (
	smCXScreen = 0
	smCYScreen = 1

	srccopy    = 0x00CC0020
	captureBlt = 0x40000000 // 把分层窗口（半透明浮层）也抓进来
	dibRGB     = 0          // 颜色表按 RGB 解释
)

// bitmapInfoHeader 即 Win32 的 BITMAPINFOHEADER（40 字节，字段顺序不能改）。
type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32 // 负数 = 自上而下（跟 image.RGBA 一致，省得翻行）
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// GrabScreenPNG 抓主显示器整屏，返回 PNG 字节与宽高（物理像素）。
func GrabScreenPNG() ([]byte, int, int, error) {
	w, _, _ := procGetSysMetrics.Call(smCXScreen)
	h, _, _ := procGetSysMetrics.Call(smCYScreen)
	if w == 0 || h == 0 {
		return nil, 0, 0, fmt.Errorf("取不到屏幕尺寸")
	}
	W, H := int(w), int(h)

	hdc, _, _ := procGetDC.Call(0)
	if hdc == 0 {
		return nil, 0, 0, fmt.Errorf("GetDC 失败（没有桌面？）")
	}
	defer procReleaseDC.Call(0, hdc)

	memDC, _, _ := procCreateCompDC.Call(hdc)
	if memDC == 0 {
		return nil, 0, 0, fmt.Errorf("CreateCompatibleDC 失败")
	}
	defer procDeleteDC.Call(memDC)

	bi := bitmapInfoHeader{
		Size:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:    int32(W),
		Height:   -int32(H), // 自上而下
		Planes:   1,
		BitCount: 32,
	}
	var bitsPtr unsafe.Pointer
	bmp, _, errNo := procCreateDIBSect.Call(hdc, uintptr(unsafe.Pointer(&bi)), dibRGB,
		uintptr(unsafe.Pointer(&bitsPtr)), 0, 0)
	if bmp == 0 || bitsPtr == nil {
		return nil, 0, 0, fmt.Errorf("CreateDIBSection 失败（错误码 %v）", errNo)
	}
	defer procDeleteObject.Call(bmp)

	old, _, _ := procSelectObject.Call(memDC, bmp)
	defer procSelectObject.Call(memDC, old)

	ok, _, errNo := procBitBlt.Call(memDC, 0, 0, uintptr(W), uintptr(H), hdc, 0, 0, uintptr(srccopy|captureBlt))
	if ok == 0 {
		return nil, 0, 0, fmt.Errorf("BitBlt 失败（错误码 %v）", errNo)
	}
	// GDI 绘图要等它落进那块内存再读；GdiFlush 是官方做法。
	procGdiFlush.Call()

	// 32bpp：每像素 4 字节，行天然按 4 对齐，所以行占就是 W*4。
	stride := W * 4
	pix := unsafe.Slice((*byte)(bitsPtr), stride*H)

	img := image.NewRGBA(image.Rect(0, 0, W, H))
	for y := 0; y < H; y++ {
		src := y * stride
		dst := y * img.Stride
		// BGRA → RGBA（image.RGBA 是 R,G,B,A 顺序）
		for x := 0; x < W*4; x += 4 {
			img.Pix[dst+x+0] = pix[src+x+2]
			img.Pix[dst+x+1] = pix[src+x+1]
			img.Pix[dst+x+2] = pix[src+x+0]
			img.Pix[dst+x+3] = 255 // 屏幕抓取没有真透明，统一不透明
		}
	}

	var out bytes.Buffer
	// 默认压缩级别最费时间。
	// 底图只是拿来当背景画，不存盘，压不压小无所谓 → 用最快档。
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&out, img); err != nil {
		return nil, 0, 0, fmt.Errorf("PNG 编码失败: %v", err)
	}
	return out.Bytes(), W, H, nil
}
