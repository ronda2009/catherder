// 服务忙闲通用识别的单元测试（busy.go）。
// 用假服务模拟 llama.cpp / ollama / llama-swap / 自家路由各答哪种接口，
// 验「没有 /status 的服务也能认出忙闲」与「都问不到就降级」。不连真服务、不占模型。
package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"catherder/internal/openclaw"
)

// fakeServer 按「哪些路径答什么」起一个假服务；没列出的路径回 404（跟真服务一样）。
// 返回这个假服务的 主机:端口，直接喂给 busyAt。
func fakeServer(t *testing.T, routes map[string]string) string {
	t.Helper()
	mux := http.NewServeMux()
	for path, body := range routes {
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, body)
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.Listener.Addr().String()
}

func newProbeEngine(t *testing.T) *Engine {
	t.Helper()
	e := newTestEngine(t)
	e.busyCache = map[string]probeAsk{} // 每个用例一份干净缓存，别被上一个用例的缓存影响
	return e
}

// 自家路由那种：有 /status。排队数大于 0 也算忙（老行为，不能退）。
func TestBusyStatusEndpoint(t *testing.T) {
	e := newProbeEngine(t)

	hp := fakeServer(t, map[string]string{"/status": `{"busy":true,"queued":0}`})
	if a := e.busyAt(hp); !a.Known || !a.Busy || a.Kind != "状态接口" {
		t.Errorf("自报忙：要 Known+Busy+状态接口，实得 %+v", a)
	}

	hp = fakeServer(t, map[string]string{"/status": `{"busy":false,"queued":3}`})
	if a := e.busyAt(hp); !a.Known || !a.Busy {
		t.Errorf("排队 3 条：要算忙，实得 %+v", a)
	}

	hp = fakeServer(t, map[string]string{"/status": `{"busy":false,"queued":0}`})
	if a := e.busyAt(hp); !a.Known || a.Busy {
		t.Errorf("自报空闲：要 Known 且不忙，实得 %+v", a)
	}
}

// llama.cpp（llama-server）：没有 /status，只有 /slots —— 任一槽在算就算忙。
// 这是老板 2026-11-05 报的那一类：切 27B 后原来一律「未知」。
func TestBusyLlamaCppSlots(t *testing.T) {
	e := newProbeEngine(t)

	hp := fakeServer(t, map[string]string{
		"/health": `{"status":"ok"}`,
		"/slots":  `[{"id":0,"is_processing":true},{"id":1,"is_processing":false}]`,
	})
	a := e.busyAt(hp)
	if !a.Known || !a.Busy || a.Kind != "llama.cpp" {
		t.Fatalf("槽在算：要 Known+Busy+llama.cpp，实得 %+v", a)
	}

	hp = fakeServer(t, map[string]string{
		"/health": `{"status":"ok"}`,
		"/slots":  `[{"id":0,"is_processing":false}]`,
	})
	if a := e.busyAt(hp); !a.Known || a.Busy || a.Kind != "llama.cpp" {
		t.Errorf("槽全空：要 Known 且不忙，实得 %+v", a)
	}

	// 配了 API 密钥时 /slots 会 401（假服务用没这个路径代替）：不能瞎判，要报探不到。
	hp = fakeServer(t, map[string]string{"/health": `{"status":"ok"}`})
	if a := e.busyAt(hp); a.Known {
		t.Errorf("只有 /health：要探不到（/health 不带忙闲），实得 %+v", a)
	}
}

// ollama：没有 /status、没有 /slots，只有 /api/ps（列已装进内存的模型）。
// 有模型驻留就占着显存 → 按忙处理；空列表算闲。
func TestBusyOllamaApiPs(t *testing.T) {
	e := newProbeEngine(t)

	hp := fakeServer(t, map[string]string{
		"/api/ps":      `{"models":[{"name":"qwen3-coder:30b","model":"qwen3-coder:30b"}]}`,
		"/api/version": `{"version":"0.14.0"}`,
	})
	a := e.busyAt(hp)
	if !a.Known || !a.Busy || a.Kind != "ollama" {
		t.Fatalf("有模型驻留：要 Known+Busy+ollama，实得 %+v", a)
	}

	hp = fakeServer(t, map[string]string{"/api/ps": `{"models":[]}`})
	if a := e.busyAt(hp); !a.Known || a.Busy || a.Kind != "ollama" {
		t.Errorf("没模型驻留：要 Known 且不忙，实得 %+v", a)
	}
}

// llama-swap（锻造台 27B 用的那套）：自己不推理，/running 报挂着的模型与它的后端地址。
// 要能追到后端那一层再问一次，认成 llama-swap→llama.cpp。
func TestBusyLlamaSwapFollowsProxy(t *testing.T) {
	e := newProbeEngine(t)

	// 后端：一个只有 /slots 的 llama-server。
	back := fakeServer(t, map[string]string{"/slots": `[{"id":0,"is_processing":true}]`})
	// 代理口：/health 只回 OK（不带忙闲），/running 报后端地址。
	front := fakeServer(t, map[string]string{
		"/health":    `OK`,
		"/running":   fmt.Sprintf(`{"running":[{"model":"j27bx","state":"ready","proxy":"http://%s"}]}`, back),
		"/v1/models": `{"data":[]}`,
	})
	a := e.busyAt(front)
	if !a.Known || !a.Busy {
		t.Fatalf("swap 后端在算：要 Known+Busy，实得 %+v", a)
	}
	if a.Kind != "llama-swap→llama.cpp" {
		t.Errorf("要认成 llama-swap→llama.cpp，实得 %q", a.Kind)
	}

	// 后端空闲：整体算闲，但种类仍要说清是隔了一层的。
	back = fakeServer(t, map[string]string{"/slots": `[{"id":0,"is_processing":false}]`})
	front = fakeServer(t, map[string]string{
		"/health":  `OK`,
		"/running": fmt.Sprintf(`{"running":[{"model":"j27bx","state":"ready","proxy":"http://%s"}]}`, back),
	})
	if a := e.busyAt(front); !a.Known || a.Busy {
		t.Errorf("swap 后端空闲：要 Known 且不忙，实得 %+v", a)
	}

	// 没挂模型：算闲（27B 没起的时候不该一直拦着）。
	front = fakeServer(t, map[string]string{"/health": `OK`, "/running": `{"running":[]}`})
	if a := e.busyAt(front); !a.Known || a.Busy || a.Kind != "llama-swap" {
		t.Errorf("swap 没挂模型：要 Known 且不忙，实得 %+v", a)
	}

	// 正在起（state 不是 ready）：算忙。
	front = fakeServer(t, map[string]string{"/health": `OK`, "/running": `{"running":[{"model":"j27bx","state":"starting","proxy":"http://127.0.0.1:1"}]}`})
	if a := e.busyAt(front); !a.Known || !a.Busy {
		t.Errorf("swap 正在起模型：要算忙，实得 %+v", a)
	}

	// 挂着模型但问不到后端（后端端口没开）：仍要知道它占着显存 → 算忙，不许报探不到。
	front = fakeServer(t, map[string]string{"/health": `OK`, "/running": `{"running":[{"model":"j27bx","state":"ready","proxy":"http://127.0.0.1:1"}]}`})
	if a := e.busyAt(front); !a.Known || !a.Busy {
		t.Errorf("挂着模型但问不到后端：要 Known+Busy，实得 %+v", a)
	}

	// proxy 指回自己：不许追成环（能返回、且仍算忙）。
	front = fakeServer(t, map[string]string{"/health": `OK`, "/running": `{"running":[{"model":"j27bx","state":"ready"}]}`})
	if a := e.busyAt(front); !a.Known || !a.Busy {
		t.Errorf("proxy 为空（指回自己）：要 Known+Busy 且不卡死，实得 %+v", a)
	}
}

// 自家路由那种「档位切换器」：/__status 报当前档在哪个端口，要问背后那一档。
// 状态地址直接指到推理服务时也要能用（问不出 __status 就当推理服务问）。
func TestBusyTierSwitcherAndDirectAddress(t *testing.T) {
	e := newProbeEngine(t)

	back := fakeServer(t, map[string]string{"/status": `{"busy":true,"queued":0}`})
	switcher := fakeServer(t, map[string]string{
		"/__status": fmt.Sprintf(`{"current":"b27bx","tiers":{"b27bx":%d}}`, portOf(back)),
	})
	if a := e.tierBusy(switcher); !a.Known || !a.Busy {
		t.Errorf("切换器背后那档在忙：要 Known+Busy，实得 %+v", a)
	}

	// 状态地址直接指到 llama.cpp（没有 __status）：要能当推理服务问出来。
	direct := fakeServer(t, map[string]string{"/slots": `[{"id":0,"is_processing":true}]`})
	if a := e.tierBusy(direct); !a.Known || !a.Busy || a.Kind != "llama.cpp" {
		t.Errorf("状态地址直接指 llama.cpp：要认成 llama.cpp 且忙，实得 %+v", a)
	}
}

// 窗口用的服务商：云端与没配的判不了（Known=false，调用方不拦）；本地的走同一套探测。
func TestBusyProviderBusyLocalOnly(t *testing.T) {
	e := newProbeEngine(t)
	hp := fakeServer(t, map[string]string{"/slots": `[{"id":0,"is_processing":true}]`})
	if e.cfg.Models.Providers == nil {
		e.cfg.Models.Providers = map[string]openclaw.ProviderEntry{}
	}
	e.cfg.Models.Providers["llamacpp"] = openclaw.ProviderEntry{BaseURL: "http://" + hp + "/v1"}

	if a := e.providerBusy("llamacpp"); !a.Known || !a.Busy {
		t.Errorf("本地服务商在忙：要 Known+Busy，实得 %+v", a)
	}
	if a := e.providerBusy("cloud"); a.Known {
		t.Errorf("云端服务商：要 Known=false（判不了就别拦），实得 %+v", a)
	}
	if a := e.providerBusy("没这个服务商"); a.Known {
		t.Errorf("没配的服务商：要 Known=false，实得 %+v", a)
	}
}

// 谁都问不到（端口上是别的东西 / 服务没起）：Known=false，调用方降级只看窗口。
func TestBusyNothingAnswers(t *testing.T) {
	e := newProbeEngine(t)
	hp := fakeServer(t, map[string]string{"/v1/models": `{"data":[]}`})
	if a := e.busyAt(hp); a.Known {
		t.Errorf("只有 /v1/models：要 Known=false，实得 %+v", a)
	}
	if a := e.busyAt(""); a.Known {
		t.Errorf("空地址：要 Known=false，实得 %+v", a)
	}
}

// 缓存：一轮里全局判一次、每个本地窗口又各判一次，同一端口只该真问一次。
func TestBusyProbeIsCached(t *testing.T) {
	e := newProbeEngine(t)
	var hits int
	mux := http.NewServeMux()
	mux.HandleFunc("/slots", func(w http.ResponseWriter, _ *http.Request) {
		hits++
		fmt.Fprint(w, `[{"id":0,"is_processing":true}]`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	hp := srv.Listener.Addr().String()

	for i := 0; i < 5; i++ {
		if a := e.busyAt(hp); !a.Busy {
			t.Fatalf("第 %d 次要 Busy，实得 %+v", i+1, a)
		}
	}
	if hits != 1 {
		t.Errorf("同一端口一轮里要只问一次，实问 %d 次", hits)
	}
}

func portOf(hostport string) int {
	_, p := splitHostPort(hostport)
	var n int
	fmt.Sscanf(p, "%d", &n)
	return n
}
