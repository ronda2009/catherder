//go:build windows

package engine

import (
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 显卡利用率（Windows）：走系统性能计数器 `\GPU Engine(*)\Utilization Percentage`。
// 这条路是**厂商中立**的 —— NVIDIA / AMD / Intel 的 WDDM 驱动都提供同一个计数器
// 所以开源出去三家机器都能用，不像 nvidia-smi 只认 N 卡。
// 用 PdhAddEnglishCounterW（英文计数器名）→ 跟系统显示语言无关，中文系统也一样读得到。

var (
	pdhDLL                   = windows.NewLazySystemDLL("pdh.dll")
	procPdhOpenQuery         = pdhDLL.NewProc("PdhOpenQueryW")
	procPdhAddEnglishCounter = pdhDLL.NewProc("PdhAddEnglishCounterW")
	procPdhCollectQueryData  = pdhDLL.NewProc("PdhCollectQueryData")
	procPdhGetFormattedArray = pdhDLL.NewProc("PdhGetFormattedCounterArrayW")
	procPdhCloseQuery        = pdhDLL.NewProc("PdhCloseQuery")
)

const (
	pdhFmtDouble = 0x00000200 // PDH_FMT_DOUBLE
	pdhMoreData  = 0x800007D2 // PDH_MORE_DATA（头一次调用是问「缓冲区要多大」，不是错误）

	pdhCstatusValid   = 0x00000000 // PDH_CSTATUS_VALID_DATA
	pdhCstatusNewData = 0x00000001 // PDH_CSTATUS_NEW_DATA
)

// pdhFmtCounterValueItem 对应 PDH_FMT_COUNTERVALUE_ITEM_W（64 位）
// 名字指针(8) + CStatus(4) + 对齐补空(4) + double 值(8) = **24 字节**。
// 步长本机（拿 24/40/48/56/64/128 逐个试，只有 24 能让 672 个实例名全部合法）。
// 注意 bufSize/itemCount ≈151 不是步长 —— 实例名另存在缓冲区后半段。
type pdhFmtCounterValueItem struct {
	Name    *uint16
	CStatus uint32
	Value   float64
}

// PDH 的利用率是「两次采样之间的平均值」，所以查询句柄要**跨轮留着**
// 每轮只采一次，读的就是「上一轮到这一轮」这段的平均占用。
var (
	gpuPdhMu      sync.Mutex
	gpuPdhQuery   uintptr
	gpuPdhCounter uintptr
	gpuPdhPrimed  bool
)

var physRe = regexp.MustCompile(`phys_(\d+)`)

// hideWindow 让兜底的 nvidia-smi 不弹黑控制台窗（界面版是 windowsgui，每轮闪一个窗很碍眼）。
func hideWindow(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

// readGpuPercent 读一次整机显卡利用率：按物理卡（实例名里的 phys_N）把各引擎占用加起来
// （封顶 100），几张卡取最高的那张 —— 跟 nvidia-smi「看最紧那张卡」口径一致。
// 计数器读不到（老驱动 / 没有 GPU Engine 计数器）→ 退回 nvidia-smi；都不行才报「读不到」。
func readGpuPercent() (int, bool) {
	if pct, ok := readGpuPercentPdh(); ok {
		return pct, true
	}
	return readNvidiaPercent()
}

// readGpuPercentPdh 走性能计数器取数。
func readGpuPercentPdh() (int, bool) {
	gpuPdhMu.Lock()
	defer gpuPdhMu.Unlock()

	if gpuPdhQuery == 0 {
		var q uintptr
		if r, _, _ := procPdhOpenQuery.Call(0, 0, uintptr(unsafe.Pointer(&q))); r != 0 {
			return 0, false
		}
		path, err := windows.UTF16PtrFromString(`\GPU Engine(*)\Utilization Percentage`)
		if err != nil {
			procPdhCloseQuery.Call(q)
			return 0, false
		}
		var c uintptr
		if r, _, _ := procPdhAddEnglishCounter.Call(q, uintptr(unsafe.Pointer(path)), 0, uintptr(unsafe.Pointer(&c))); r != 0 {
			procPdhCloseQuery.Call(q)
			return 0, false
		}
		gpuPdhQuery, gpuPdhCounter, gpuPdhPrimed = q, c, false
	}

	// 头一次：先采一版当基线，等半秒再采一版，才有「这段时间的平均值」。
	if !gpuPdhPrimed {
		if r, _, _ := procPdhCollectQueryData.Call(gpuPdhQuery); r != 0 {
			closeGpuPdhLocked()
			return 0, false
		}
		time.Sleep(500 * time.Millisecond)
		gpuPdhPrimed = true
	}
	if r, _, _ := procPdhCollectQueryData.Call(gpuPdhQuery); r != 0 {
		closeGpuPdhLocked()
		return 0, false
	}

	items, ok := readGpuPdhItemsLocked()
	if !ok {
		return 0, false
	}

	perPhys := map[string]float64{}
	for i := range items {
		if items[i].CStatus != pdhCstatusValid && items[i].CStatus != pdhCstatusNewData {
			continue
		}
		name := windows.UTF16PtrToString(items[i].Name)
		if name == "" || items[i].Value <= 0 {
			continue
		}
		key := "0"
		if m := physRe.FindStringSubmatch(strings.ToLower(name)); len(m) == 2 {
			key = m[1]
		}
		perPhys[key] += items[i].Value
	}
	if len(perPhys) == 0 {
		return 0, true // 计数器在、但没任何进程在用显卡 = 空闲
	}
	top := 0.0
	for _, v := range perPhys {
		if v > 100 {
			v = 100 // 同一张卡上多个引擎的占用不能加超过 100%
		}
		if v > top {
			top = v
		}
	}
	pct := int(top + 0.5)
	if pct > 100 {
		pct = 100
	}
	return pct, true
}

// readGpuPdhItemsLocked 取这一轮所有实例的读数（缓冲区大小要先问一次）。
func readGpuPdhItemsLocked() ([]pdhFmtCounterValueItem, bool) {
	var bufSize, itemCount uint32
	r, _, _ := procPdhGetFormattedArray.Call(gpuPdhCounter, pdhFmtDouble,
		uintptr(unsafe.Pointer(&bufSize)), uintptr(unsafe.Pointer(&itemCount)), 0)
	if r != pdhMoreData && r != 0 {
		return nil, false
	}
	if bufSize == 0 || itemCount == 0 {
		return nil, false
	}
	buf := make([]byte, bufSize)
	r, _, _ = procPdhGetFormattedArray.Call(gpuPdhCounter, pdhFmtDouble,
		uintptr(unsafe.Pointer(&bufSize)), uintptr(unsafe.Pointer(&itemCount)), uintptr(unsafe.Pointer(&buf[0])))
	if r != 0 {
		return nil, false
	}
	if itemCount == 0 {
		return nil, true
	}
	stride := unsafe.Sizeof(pdhFmtCounterValueItem{})
	if uint64(itemCount)*uint64(stride) > uint64(bufSize) {
		itemCount = uint32(uint64(bufSize) / uint64(stride)) // 兜住：别越界读
	}
	out := make([]pdhFmtCounterValueItem, 0, itemCount)
	for i := uint32(0); i < itemCount; i++ {
		out = append(out, *(*pdhFmtCounterValueItem)(unsafe.Add(unsafe.Pointer(&buf[0]), uintptr(i)*stride)))
	}
	return out, true
}

// closeGpuPdhLocked 关掉查询；下次读时重开（驱动换了也能自愈）。
func closeGpuPdhLocked() {
	if gpuPdhQuery != 0 {
		procPdhCloseQuery.Call(gpuPdhQuery)
	}
	gpuPdhQuery, gpuPdhCounter, gpuPdhPrimed = 0, 0, false
}
