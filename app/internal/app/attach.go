package app

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// 传文件：选或拖多个文件，随任务一起发给目标窗口（走网关 sessions.send 的 attachments）。

// 网关能收的文档类型（超出这些的照发但对方可能不认，界面里先提示一句）。
var docExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true,
	".pdf": true, ".txt": true, ".md": true, ".markdown": true, ".csv": true, ".tsv": true,
	".json": true, ".yaml": true, ".yml": true, ".log": true,
	".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
	".zip": true,
}

var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true,
}

// maxFileBytes 单个附件上限。网关单图上限 6MB，这里留余量并给明确提示。
const maxFileBytes = 5 * 1024 * 1024

// PickFiles 弹系统文件选择框（可多选），返回选中的路径；用户取消返回空表。
func (a *App) PickFiles() ([]string, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("界面还没起来，选不了文件")
	}
	ps, err := wruntime.OpenMultipleFilesDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:            uiT("选要随任务发出的文件（可多选）"),
		DefaultDirectory: pickStartDir(),
	})
	if err != nil {
		return nil, fmt.Errorf("打开文件选择框失败：%v", err)
	}
	return ps, nil
}

// CheckFiles 逐个看这个文件能不能发；全都能发返回空串，否则返回要显示的话。
func (a *App) CheckFiles(paths []string) string {
	var bad, odd []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		fi, err := os.Stat(p)
		if err != nil {
			bad = append(bad, filepath.Base(p)+"（读不到）")
			continue
		}
		if fi.IsDir() {
			bad = append(bad, filepath.Base(p)+"（是文件夹）")
			continue
		}
		if fi.Size() > maxFileBytes {
			bad = append(bad, fmt.Sprintf("%s（%d MB，超上限 5 MB）", filepath.Base(p), fi.Size()/1024/1024))
			continue
		}
		if !docExts[strings.ToLower(filepath.Ext(p))] {
			odd = append(odd, filepath.Base(p))
		}
	}
	if len(bad) > 0 {
		return "发不出去：" + strings.Join(bad, "、")
	}
	if len(odd) > 0 {
		return "这几个类型对方可能不认（照发）：" + strings.Join(odd, "、")
	}
	return ""
}

// AddTaskFiles 把一条带多个附件的任务加入队列。paths 为空等于普通任务。
func (a *App) AddTaskFiles(targetKey, body string, paths []string) (int, error) {
	clean := make([]string, 0, len(paths))
	for _, p := range paths {
		if p != "" {
			clean = append(clean, p)
		}
	}
	if len(clean) == 0 {
		return a.AddTask(targetKey, body)
	}
	if warn := a.CheckFiles(clean); strings.Contains(warn, "发不出去") {
		return 0, fmt.Errorf("%s", warn)
	}
	id, err := a.AddTask(targetKey, body)
	if err != nil {
		return 0, err
	}
	if err := a.st.SetAttachPaths(id, clean); err != nil {
		a.CancelTask(id)
		return 0, fmt.Errorf("记录附件失败：%v", err)
	}
	parts := make([]string, 0, len(clean))
	for _, p := range clean {
		sizeKB := int64(0)
		if fi, serr := os.Stat(p); serr == nil {
			sizeKB = fi.Size() / 1024
		}
		parts = append(parts, fmt.Sprintf("%s(%d KB)", filepath.Base(p), sizeKB))
	}
	a.st.Log("attach", "T-%03d 已附 %d 个文件：%s", id, len(clean), strings.Join(parts, "、"))
	return id, nil
}

// CopyText 把文字写进剪贴板（截图取色器「Ctrl+C 复制色值」用）。
func (a *App) CopyText(s string) error {
	if a.ctx == nil {
		return fmt.Errorf("界面还没起来")
	}
	return wruntime.ClipboardSetText(a.ctx, s)
}

// ---- 缩略图 ----

type thumbRec struct {
	mtime int64
	data  string
}

var thumbs sync.Map // 路径 → thumbRec；大图解码一次就够，队列每次刷新都调它

// Thumb 图片缩略图（最长边 240）的 data URL；非图片、读不到或解不开返回空串。
func (a *App) Thumb(path string) string {
	if path == "" || !imageExts[strings.ToLower(filepath.Ext(path))] {
		return ""
	}
	fi, err := os.Stat(path)
	if err != nil {
		return ""
	}
	mt := fi.ModTime().UnixNano()
	if v, ok := thumbs.Load(path); ok {
		if rec, ok2 := v.(thumbRec); ok2 && rec.mtime == mt {
			return rec.data
		}
	}
	data, derr := makeThumb(path)
	if derr != nil {
		return ""
	}
	thumbs.Store(path, thumbRec{mtime: mt, data: data})
	return data
}

func makeThumb(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	cfg, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}
	const maxSide = 240
	b := cfg.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return "", fmt.Errorf("图片尺寸不对")
	}
	out := image.NewRGBA(b)
	if w > maxSide || h > maxSide {
		s := float64(maxSide) / float64(max(w, h))
		tw, th := int(float64(w)*s+0.5), int(float64(h)*s+0.5)
		if tw < 1 {
			tw = 1
		}
		if th < 1 {
			th = 1
		}
		out = image.NewRGBA(image.Rect(0, 0, tw, th))
		for y := 0; y < th; y++ {
			sy := b.Min.Y + int(float64(y)/s)
			if sy >= b.Max.Y {
				sy = b.Max.Y - 1
			}
			for x := 0; x < tw; x++ {
				sx := b.Min.X + int(float64(x)/s)
				if sx >= b.Max.X {
					sx = b.Max.X - 1
				}
				out.Set(x, y, cfg.At(sx, sy))
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// FileData 原文件的 data URL（点缩略图放大时用）；非图片类型返回空串。
func (a *App) FileData(path string) string {
	if path == "" || !imageExts[strings.ToLower(filepath.Ext(path))] {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) > maxFileBytes {
		return ""
	}
	return "data:" + mimeOf(path) + ";base64," + base64.StdEncoding.EncodeToString(raw)
}

func mimeOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".webp":
		return "image/webp"
	}
	return "application/octet-stream"
}

// pickStartDir 默认落在文档目录，比落在程序目录好找。
func pickStartDir() string {
	for _, k := range []string{"USERPROFILE", "HOMEPATH"} {
		if v := os.Getenv(k); v != "" {
			d := filepath.Join(v, "Documents")
			if fi, err := os.Stat(d); err == nil && fi.IsDir() {
				return d
			}
			return v
		}
	}
	return ""
}
