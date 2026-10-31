// 命令行截图：catherder shot [存到哪]
// 为什么命令行也要有：抓屏那段是 Win32 GDI（BitBlt/GetDIBits），最容易出「看着跑通了其实抓到黑图」的问题。
// 命令行能直接抓一张、报尺寸与字节数，先验证抓屏本身对不对，再去调界面那层。
package cli

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"catherder/internal/screen"
	"catherder/internal/store"
)

func cmdShot(st *store.Store, args []string) {
	out := ""
	if len(args) > 0 {
		out = args[0]
	}
	pngBytes, w, h, err := screen.GrabScreenPNG()
	if err != nil {
		die("抓屏失败：%v", err)
	}
	// 自己解一遍 PNG：尺寸对不上或整张是纯色，说明抓的是黑帧（GDI 走错的典型表现）。
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		die("抓回来的不是有效 PNG：%v", err)
	}
	bd := img.Bounds()
	if bd.Dx() != w || bd.Dy() != h {
		die("尺寸不一致：报 %dx%d，解出来 %dx%d", w, h, bd.Dx(), bd.Dy())
	}
	// 抽样看有没有内容（取 9 个点，统计不同颜色数）
	seen := map[uint32]bool{}
	for _, p := range [][2]int{{2, 2}, {bd.Dx() / 2, 2}, {bd.Dx() - 3, 2}, {2, bd.Dy() / 2}, {bd.Dx() / 2, bd.Dy() / 2}, {bd.Dx() - 3, bd.Dy() / 2}, {2, bd.Dy() - 3}, {bd.Dx() / 2, bd.Dy() - 3}, {bd.Dx() - 3, bd.Dy() - 3}} {
		r, g, b, _ := img.At(p[0], p[1]).RGBA()
		seen[(uint32(r)>>4<<16)|(uint32(g)>>4<<8)|(uint32(b)>>4)] = true
	}
	if out == "" {
		dir := filepath.Join(filepath.Dir(store.DefaultPath()), "截图")
		os.MkdirAll(dir, 0o755)
		out = filepath.Join(dir, fmt.Sprintf("截图-%s.png", time.Now().Format("20060102-150405")))
	}
	if err := os.WriteFile(out, pngBytes, 0o644); err != nil {
		die("存图失败：%v", err)
	}
	fmt.Printf("抓屏 OK：%dx%d 物理像素，PNG %d KB，抽样 %d 种颜色 → %s\n",
		w, h, len(pngBytes)/1024, len(seen), out)
	if len(seen) <= 1 {
		fmt.Println("⚠️ 抽样全是同一颜色，可能抓到的是黑帧/纯色，别当成功。")
	}
}
