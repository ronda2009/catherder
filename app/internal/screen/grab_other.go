//go:build !windows

// 非 Windows 平台没有抓屏实现（本程序只在 Windows 上跑界面版）
// 留一个同名桩，好让包在别的系统上也能编译与测试。
package screen

import "fmt"

func GrabScreenPNG() ([]byte, int, int, error) {
	return nil, 0, 0, fmt.Errorf("这个平台还不支持抓屏")
}
