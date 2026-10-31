package app

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePNG(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	p := filepath.Join(dir, name)
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatalf("编图失败: %v", err)
	}
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("写图失败: %v", err)
	}
	return p
}

func TestCheckFiles(t *testing.T) {
	dir := t.TempDir()
	img := writePNG(t, dir, "a.png", 40, 30)
	odd := filepath.Join(dir, "x.rar")
	if err := os.WriteFile(odd, []byte("x"), 0o644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}
	big := filepath.Join(dir, "big.bin")
	if err := os.WriteFile(big, make([]byte, maxFileBytes+1), 0o644); err != nil {
		t.Fatalf("写大文件失败: %v", err)
	}

	a := &App{}
	if w := a.CheckFiles([]string{img}); w != "" {
		t.Errorf("正常图片不该报问题: %q", w)
	}
	if w := a.CheckFiles([]string{img, dir}); strings.Contains(w, "发不出去") == false {
		t.Errorf("文件夹该被挡: %q", w)
	}
	if w := a.CheckFiles([]string{big}); !strings.HasPrefix(w, "发不出去") {
		t.Errorf("超上限该被挡: %q", w)
	}
	if w := a.CheckFiles([]string{odd}); !strings.Contains(w, "可能不认") {
		t.Errorf("未知类型该只提示不挡: %q", w)
	}
	if w := a.CheckFiles([]string{filepath.Join(dir, "没了.png")}); !strings.Contains(w, "读不到") {
		t.Errorf("不存在的文件该报读不到: %q", w)
	}
}

func TestThumbAndFileData(t *testing.T) {
	dir := t.TempDir()
	a := &App{}

	big := writePNG(t, dir, "big.png", 900, 600)
	u := a.Thumb(big)
	if !strings.HasPrefix(u, "data:image/png;base64,") {
		t.Fatalf("缩略图没出来: %q", u[:min(len(u), 40)])
	}
	cfg, _, err := image.Decode(bytes.NewReader(bytesTrim(u)))
	if err != nil {
		t.Fatalf("缩略图解不开: %v", err)
	}
	if b := cfg.Bounds(); b.Dx() > 240 || b.Dy() > 240 {
		t.Errorf("缩略图没缩到 240 以内: %dx%d", b.Dx(), b.Dy())
	}

	small := writePNG(t, dir, "small.png", 60, 40)
	if u2 := a.Thumb(small); !strings.HasPrefix(u2, "data:image/png;base64,") {
		t.Errorf("小图也该出缩略图")
	}

	if a.Thumb(filepath.Join(dir, "a.txt")) != "" {
		t.Errorf("非图片不该出缩略图")
	}
	if a.Thumb(filepath.Join(dir, "没了.png")) != "" {
		t.Errorf("文件不在该返回空串")
	}

	d := a.FileData(big)
	if !strings.HasPrefix(d, "data:image/png;base64,") {
		t.Errorf("原图 data URL 没出来")
	}
	if a.FileData(filepath.Join(dir, "x.pdf")) != "" {
		t.Errorf("非图片类型 FileData 该返回空串")
	}
}

func bytesTrim(dataURL string) []byte {
	i := strings.Index(dataURL, ",")
	if i < 0 {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(dataURL[i+1:])
	if err != nil {
		return nil
	}
	return raw
}
