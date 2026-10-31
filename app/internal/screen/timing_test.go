package screen

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"testing"
	"time"
)

// 抓屏编码档位对比：编码耗时 vs 字节数（字节大 = 传给界面解码就慢）。
func TestEncodeTiming(t *testing.T) {
	raw, w, h, err := GrabScreenPNG()
	if err != nil {
		t.Skipf("抓屏不可用：%v", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("解不开：%v", err)
	}
	t.Logf("屏幕 %dx%d 原始 RGBA %d MB", w, h, w*h*4/1024/1024)

	for _, tc := range []struct {
		name string
		lvl  png.CompressionLevel
	}{
		{"默认(旧)", png.DefaultCompression},
		{"最快(现)", png.BestSpeed},
	} {
		var bestTime time.Duration
		var size int
		for i := 0; i < 3; i++ {
			t0 := time.Now()
			var out bytes.Buffer
			enc := png.Encoder{CompressionLevel: tc.lvl}
			if err := enc.Encode(&out, img); err != nil {
				t.Fatalf("%s 编码失败：%v", tc.name, err)
			}
			el := time.Since(t0)
			if i == 0 || el < bestTime {
				bestTime = el
			}
			size = out.Len()
		}
		b64 := base64.StdEncoding.EncodeToString(make([]byte, size))
		t.Logf("%s 编码 %v → %d KB（base64 %d KB）", tc.name, bestTime.Round(time.Millisecond), size/1024, len(b64)/1024)
	}
	_ = image.RGBA{}
}
