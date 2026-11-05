// 模型服务忙闲的通用识别。
//
// 背景：原来只认档位切换器的 /__status 加推理服务的 /status —— 那是自家路由写的接口，不是行业标准。
// 别人机器上常见的 llama.cpp、ollama、llama-swap 都没有 /status，于是全部「探不到」，
// 服务闸门形同不存在（2026-11-05 老板 T-139：切 27B 后界面显示「未知」、判断不成立）。
//
// 做法：同一个端口依次试几种问法，谁答就用谁；都不答就 Known=false，调用方降级只看窗口（老行为）。
// 各家的问法与字段名都按上游文档/源码核对过（实测记录见 tools\脚本索引.md）：
//   - llama.cpp llama-server：GET /slots → 槽数组，任一 is_processing 就是在算（默认开，--no-slots 可关）
//   - ollama：GET /api/ps → {"models":[…]}，列的是已装进内存的模型（驻留就占着显存）
//   - llama-swap：GET /running → {"running":[{"state":…,"proxy":"http://主机:端口"}]}，
//                 它自己不推理，拿 proxy 再问那一层（它代管的就是 llama-server / ollama 等）
//   - 有 /status（{"busy":…,"queued":…}）的先问：自家路由与不少代理都有，且比「有模型驻留」更准
// ollama 没有「忙」这个口（/api/status 讲的是云端账号，不是忙闲），只能判有没有模型驻留。

package engine

import (
	"fmt"
	"strings"
	"time"
)

// busyCacheTTL 是「这个端口上是什么服务、忙不忙」的缓存时长。
// 一轮里全局判一次、每个本地窗口又各判一次，不缓存请求数会乘上窗口数；引擎本身 60 秒一轮，15 秒不算旧。
const busyCacheTTL = 15 * time.Second

// maxSwapHops 是往 llama-swap 下面追几层。1 层够（swap → 它代管的那个服务）；
// 写死层数是为了：有人把 proxy 地址指回 swap 自己时不会无限追下去。
const maxSwapHops = 1

// busyAsk 是一次探测的结果。Known=false 表示这个端口上什么问法都问不到。
type busyAsk struct {
	Busy  bool   // 忙不忙（Known=false 时别看这个字段）
	Kind  string // 认出来的服务种类，界面「服务」那一栏用它，让人知道它认成了什么
	Why   string // 人话：凭什么说它忙/闲（进日志与悬停说明）
	Known bool
}

func unknownAsk() busyAsk { return busyAsk{Known: false} }

// probeAsk 是缓存里的一条：什么时候探的、探出了什么。探不到也缓存，别每轮都对没服务的端口重试一串接口。
type probeAsk struct {
	at  time.Time
	ask busyAsk
}

// busyCacheKey 归一主机名（localhost / ::1 / [::1] 都当 127.0.0.1），免得同一个服务出现三条缓存。
func busyCacheKey(hostport string) string {
	h, p := splitHostPort(hostport)
	switch strings.ToLower(h) {
	case "localhost", "::1", "[::1]":
		h = "127.0.0.1"
	}
	return h + ":" + p
}

func splitHostPort(hostport string) (string, string) {
	if i := strings.LastIndex(hostport, ":"); i >= 0 {
		return hostport[:i], hostport[i+1:]
	}
	return hostport, ""
}

// busyAt 问「这个主机:端口上的推理服务忙不忙」，自动认它是哪种服务、用它的问法。结果缓存 busyCacheTTL。
func (e *Engine) busyAt(hostport string) busyAsk {
	if hostport == "" {
		return unknownAsk()
	}
	key := busyCacheKey(hostport)
	e.busyMu.Lock()
	defer e.busyMu.Unlock()
	if p, ok := e.busyCache[key]; ok && time.Since(p.at) < busyCacheTTL {
		return p.ask
	}
	p := e.probeBusy(hostport, 0)
	e.busyCache[key] = p
	return p.ask
}

// probeBusy 依次试各种问法，谁答用谁。调用方必须已持有 busyMu：追 llama-swap 往下层时要复用同一份缓存。
func (e *Engine) probeBusy(hostport string, hop int) probeAsk {
	ask := statusBusy(hostport)
	if !ask.Known {
		ask = slotsBusy(hostport)
	}
	if !ask.Known {
		ask = ollamaBusy(hostport)
	}
	if !ask.Known && hop < maxSwapHops {
		ask = e.swapBusy(hostport, hop)
	}
	return probeAsk{at: time.Now(), ask: ask}
}

// statusBusy 问 /status（{"busy":bool,"queued":int}）：自家路由与不少代理都有这个口。
func statusBusy(hostport string) busyAsk {
	var s struct {
		Busy   *bool `json:"busy"`
		Queued *int  `json:"queued"`
	}
	if !getJSON(fmt.Sprintf("http://%s/status", hostport), &s) || s.Busy == nil {
		return unknownAsk()
	}
	q := 0
	if s.Queued != nil {
		q = *s.Queued
	}
	switch {
	case *s.Busy:
		return busyAsk{Busy: true, Known: true, Kind: "状态接口", Why: "服务自报正在生成"}
	case q > 0:
		return busyAsk{Busy: true, Known: true, Kind: "状态接口", Why: fmt.Sprintf("服务在排队 %d 条", q)}
	default:
		return busyAsk{Known: true, Kind: "状态接口", Why: "服务自报空闲"}
	}
}

// slotsBusy 问 llama.cpp 的 /slots：槽数组里任一 is_processing 就是在算。
// 配了 API 密钥时不带密钥会 401/403，getJSON 只认 200，就当问不到（不瞎猜）。
func slotsBusy(hostport string) busyAsk {
	var slots []struct {
		Processing *bool `json:"is_processing"`
	}
	if !getJSON(fmt.Sprintf("http://%s/slots", hostport), &slots) || len(slots) == 0 {
		return unknownAsk()
	}
	n := 0
	for _, s := range slots {
		if s.Processing != nil && *s.Processing {
			n++
		}
	}
	if n > 0 {
		return busyAsk{Busy: true, Known: true, Kind: "llama.cpp", Why: fmt.Sprintf("%d/%d 个槽正在生成", n, len(slots))}
	}
	return busyAsk{Known: true, Kind: "llama.cpp", Why: "槽全空"}
}

// ollamaBusy 问 ollama 的 /api/ps：它列的是已装进内存的模型。
// 模型驻留就占着显存，这时再派新任务给别的本地窗口最容易撞卡，所以按忙处理（ollama 问不到「正在生成」）。
func ollamaBusy(hostport string) busyAsk {
	var ps struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if !getJSON(fmt.Sprintf("http://%s/api/ps", hostport), &ps) {
		return unknownAsk()
	}
	if len(ps.Models) == 0 {
		return busyAsk{Known: true, Kind: "ollama", Why: "没有模型驻留"}
	}
	first := ps.Models[0].Name
	if first == "" {
		first = ps.Models[0].Model
	}
	if len(ps.Models) == 1 {
		return busyAsk{Busy: true, Known: true, Kind: "ollama", Why: fmt.Sprintf("模型 %s 驻留内存", first)}
	}
	return busyAsk{Busy: true, Known: true, Kind: "ollama", Why: fmt.Sprintf("%d 个模型驻留内存（%s 等）", len(ps.Models), first)}
}

// swapBusy 问 llama-swap 的 /running，再问它代管的那个后端的口。
// 条目 state 不是 ready（正在起、正在停）算忙；问不到后端时，「挂着模型」这件事本身已说明占着显存，按忙处理。
func (e *Engine) swapBusy(hostport string, hop int) busyAsk {
	var v struct {
		Running []struct {
			Model string `json:"model"`
			State string `json:"state"`
			Proxy string `json:"proxy"`
		} `json:"running"`
	}
	if !getJSON(fmt.Sprintf("http://%s/running", hostport), &v) {
		return unknownAsk()
	}
	if len(v.Running) == 0 {
		return busyAsk{Known: true, Kind: "llama-swap", Why: "没挂模型"}
	}
	for _, r := range v.Running {
		if r.State != "" && r.State != "ready" {
			return busyAsk{Busy: true, Known: true, Kind: "llama-swap", Why: fmt.Sprintf("模型 %s 还没挂稳（状态 %s）", r.Model, r.State)}
		}
	}
	for _, r := range v.Running {
		back := trimScheme(r.Proxy)
		if back == "" || busyCacheKey(back) == busyCacheKey(hostport) {
			continue // 有的配置把 proxy 指回自己，别追成环
		}
		a := e.probeBusy(back, hop+1).ask
		if !a.Known {
			continue
		}
		// 认出来的种类带上两层（llama-swap→llama.cpp），让人知道中间隔了一层代理；
		// 依据直接用那一层自己报的，不再拼前缀（拼出来的句子英文不好翻）。
		a.Kind = "llama-swap→" + a.Kind
		if !a.Busy {
			a.Why = "挂着的模型空闲"
		}
		return a
	}
	first := v.Running[0].Model
	if len(v.Running) > 1 {
		return busyAsk{Busy: true, Known: true, Kind: "llama-swap", Why: fmt.Sprintf("挂着 %d 个模型（%s 等），问不到它们忙不忙", len(v.Running), first)}
	}
	return busyAsk{Busy: true, Known: true, Kind: "llama-swap", Why: fmt.Sprintf("挂着模型 %s，问不到它忙不忙", first)}
}

// trimScheme 去掉 http(s):// 前缀、路径与尾斜杠，只留 主机:端口。
func trimScheme(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimPrefix(strings.TrimPrefix(u, "http://"), "https://")
	u = strings.Trim(u, "/")
	if i := strings.Index(u, "/"); i >= 0 {
		u = u[:i]
	}
	return u
}
