// spike2: M1d 尖测 —— 外部程序怎么把消息送进指定会话。
// 两条路都实测：A) HTTP /tools/invoke（文档说 sessions_send 默认在 HTTP 硬拒绝表里
// 要靠 gateway.tools.allow 放行——本机没配，预期 404，实测确认）；
// B) WebSocket RPC sessions.send（CLI 走的就是这条，token 鉴权）。
// 用不存在的会话键试探：404=工具被拒，400/404-not-found 类=工具活着但找不到会话。
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
)

type gwConfig struct {
	Gateway struct {
		Port int `json:"port"`
		Auth struct {
			Mode  string `json:"mode"`
			Token string `json:"token"`
		} `json:"auth"`
	} `json:"gateway"`
}

func loadCfg(path string) (gwConfig, error) {
	var c gwConfig
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

func main() {
	cfgPath := flag.String("config", `C:\Users\Administrator\.openclaw\openclaw.json`, "openclaw.json 路径")
	bogus := flag.String("bogus", "agent:main:dashboard:spike-nonexistent-0000", "不存在的会话键，避免真发消息打扰用户")
	flag.Parse()

	cfg, err := loadCfg(*cfgPath)
	if err != nil {
		fmt.Println("CFG FAIL:", err)
		os.Exit(1)
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", cfg.Gateway.Port)
	token := cfg.Gateway.Auth.Token
	fmt.Printf("gateway=%s auth.mode=%s token=%d chars\n", base, cfg.Gateway.Auth.Mode, len(token))

	// A1) 基线：sessions_list 应当 200
	code, body := invoke(base, token, "sessions_list", map[string]any{})
	fmt.Println("HTTP sessions_list ->", code, clip(body, 100))

	// A2) sessions_send 在默认硬拒绝表里：预期 404（本机 gateway.tools 未配 allow）
	code, body = invoke(base, token, "sessions_send", map[string]any{
		"sessionKey": *bogus, "message": "spike probe", "timeoutSeconds": 0,
	})
	fmt.Println("HTTP sessions_send ->", code, clip(body, 220))

	// B) WebSocket RPC sessions.send
	wsCode, wsBody := wsCall(cfg.Gateway.Port, token, "sessions.send", map[string]any{
		"key": *bogus, "message": "spike probe",
	})
	fmt.Println("WS sessions.send ->", wsCode, clip(wsBody, 260))
	fmt.Println("SPIKE2 DONE")
}

func invoke(base, token, tool string, args map[string]any) (int, string) {
	payload, _ := json.Marshal(map[string]any{"tool": tool, "args": args})
	req, err := http.NewRequest("POST", base+"/tools/invoke", bytes.NewReader(payload))
	if err != nil {
		return -1, err.Error()
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	cl := &http.Client{Timeout: 15 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return -1, "ERR " + err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, string(b)
}

// wsCall 复刻 CLI 的调用路径：connect(token) → 目标方法。不带设备签名，实测网关是否接受。
func wsCall(port int, token, method string, params map[string]any) (int, string) {
	url := fmt.Sprintf("ws://127.0.0.1:%d", port)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return -1, "DIAL ERR " + err.Error()
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(20 * time.Second))

	// 帧1：预连接挑战
	if _, raw, err := conn.ReadMessage(); err != nil {
		return -1, "CHALLENGE READ ERR " + err.Error()
	} else {
		fmt.Println("WS frame1:", clip(string(raw), 90))
	}

	// 帧2：connect（只带 token，不带 device 签名）
	connect := map[string]any{
		"type": "req", "id": "c1", "method": "connect",
		"params": map[string]any{
			"minProtocol": 4, "maxProtocol": 4,
			// 文档：client.id="gateway-client" + mode="backend" 的环回共享 token 连接可免设备签名
			"client": map[string]any{"id": "gateway-client", "version": "0.0.1", "platform": "windows", "mode": "backend"},
			"role":   "operator",
			// 上一轮实测：不报 scopes 时网关只给隐式读权限，sessions.send 回 MISSING_SCOPE
			"scopes": []string{"operator.read", "operator.write"},
			"auth":   map[string]any{"token": token},
		},
	}
	if err := conn.WriteJSON(connect); err != nil {
		return -1, "CONNECT WRITE ERR " + err.Error()
	}

	// 读直到 connect 的应答
	var hello map[string]any
	for {
		var f map[string]any
		if err := conn.ReadJSON(&f); err != nil {
			return -1, "HELLO READ ERR " + err.Error()
		}
		if f["type"] == "res" && f["id"] == "c1" {
			hello = f
			break
		}
	}
	ok, _ := hello["ok"].(bool)
	if !ok {
		return 1, "CONNECT REJECTED: " + clip(fmt.Sprint(hello["error"]), 200)
	}
	pl, _ := hello["payload"].(map[string]any)
	fmt.Println("WS hello-ok protocol:", pl["protocol"])

	// 帧3：目标方法
	if err := conn.WriteJSON(map[string]any{"type": "req", "id": "m1", "method": method, "params": params}); err != nil {
		return -1, "METHOD WRITE ERR " + err.Error()
	}
	for {
		var f map[string]any
		if err := conn.ReadJSON(&f); err != nil {
			return -1, "METHOD READ ERR " + err.Error()
		}
		if f["type"] == "res" && f["id"] == "m1" {
			o, _ := f["ok"].(bool)
			if o {
				return 0, "OK " + clip(fmt.Sprint(f["payload"]), 200)
			}
			return 2, "ERR " + clip(fmt.Sprint(f["error"]), 240)
		}
	}
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

var _ = filepath.Join
