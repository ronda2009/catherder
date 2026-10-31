// 截图功能对界面暴露的四个方法。
// 分工：Go 只做四件事——抓整屏、进/出全屏、把标注好的图落盘。
// 选区与画圈/写字全在界面画布里做（复用现成 WebView；Wails v2 不支持第二个窗口，
// 自己再写一套 Win32 绘图窗是上一轮被否掉的「往上堆东西」路子）。
package app

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"catherder/internal/screen"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Shot 是给界面的一张整屏图：base64 的 PNG + 物理像素宽高。
type Shot struct {
	B64    string `json:"b64"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// CaptureScreen 只抓整屏那一帧，不全屏。
// 全屏交给界面在底图解开之后才要（EnterShot）：先全屏再解码，
// 中间那一瞬看到的是「界面被撑大」，就是老板报的「闪一下 + 放大缩小卡顿」。
func (a *App) CaptureScreen() (Shot, error) {
	png, w, h, err := screen.GrabScreenPNG()
	if err != nil {
		return Shot{}, err
	}
	return Shot{
		B64:    base64.StdEncoding.EncodeToString(png),
		Width:  w,
		Height: h,
	}, nil
}

// EnterShot 把自己全屏。界面拿到底图并画好之后才调。
func (a *App) EnterShot() {
	if a.ctx != nil && !wruntime.WindowIsFullscreen(a.ctx) {
		wruntime.WindowFullscreen(a.ctx)
	}
}

// ExitShot 退出全屏，回到正常窗口。取消截图时也要调，否则窗口一直占着屏幕。
func (a *App) ExitShot() {
	if a.ctx != nil && wruntime.WindowIsFullscreen(a.ctx) {
		wruntime.WindowUnfullscreen(a.ctx)
	}
}

// SaveShot 把界面标注完的图（dataURL 或纯 base64）存进截图目录，返回落盘路径。
// 存这里：跟自己的库同目录（--data 挪哪儿就跟到哪儿）， portable 测试不用改代码。
func (a *App) SaveShot(dataURL string) (string, error) {
	if a.imgDir == "" {
		return "", fmt.Errorf("没配截图目录")
	}
	b64 := dataURL
	if i := strings.Index(b64, ","); i >= 0 && strings.HasPrefix(b64, "data:") {
		b64 = b64[i+1:]
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("图片数据解不开：%v", err)
	}
	// 网关单图上限 6MB（agent-hydration 上限），超了发送会被拒，这里先挡住并说清。
	if len(raw) > 5*1024*1024 {
		return "", fmt.Errorf("这张图 %d MB，超过发送上限 5 MB，请少画点或截小一点", len(raw)/1024/1024)
	}
	if err := os.MkdirAll(a.imgDir, 0o755); err != nil {
		return "", fmt.Errorf("建截图目录失败：%v", err)
	}
	name := fmt.Sprintf("截图-%s.png", time.Now().Format("20060102-150405"))
	p := filepath.Join(a.imgDir, name)
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		return "", fmt.Errorf("存图失败：%v", err)
	}
	a.st.Log("shot", "截图已存 %s（%d KB）", name, len(raw)/1024)
	return p, nil
}

// SaveShotAs 弹系统「另存为」对话框让老板选地方存（原来只闷在程序目录里，老板报「保存下载那个功能没有用」）。
// 没选（取消）就返回空串，界面据此说「没存」。
func (a *App) SaveShotAs(dataURL string) (string, error) {
	b64 := dataURL
	if i := strings.Index(b64, ","); i >= 0 && strings.HasPrefix(b64, "data:") {
		b64 = b64[i+1:]
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("图片数据解不开：%v", err)
	}
	name := fmt.Sprintf(uiT("截图-")+"%s.png", time.Now().Format("20060102-150405"))
	p, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title:           uiT("另存为"),
		DefaultFilename: name,
		Filters:         []wruntime.FileFilter{{DisplayName: uiT("PNG 图片"), Pattern: "*.png"}},
	})
	if err != nil {
		return "", fmt.Errorf("另存为对话框失败：%v", err)
	}
	if p == "" {
		return "", nil // 老板在对话框里取消了
	}
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		return "", fmt.Errorf("存图失败：%v", err)
	}
	a.st.Log("shot", "截图另存为 %s（%d KB）", p, len(raw)/1024)
	return p, nil
}
