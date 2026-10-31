// spike5: 转发通道的最后一环 —— conversations.send 要 agentId + conversationRef
// 实测 conversations.list 能不能拿到用户私聊的 ref（拿不到就得回退 chat.send 方案）。
// 只读探针：list 不发消息；send 用坏 ref 验证参数形状，不打扰用户。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	cfgPath := flag.String("config", `C:\Users\Administrator\.openclaw\openclaw.json`, "")
	flag.Parse()
	var cfg struct {
		Gateway struct {
			Port int                    `json:"port"`
			Auth struct{ Token string } `json:"auth"`
		} `json:"gateway"`
	}
	b, err := os.ReadFile(*cfgPath)
	if err != nil {
		fmt.Println("CFG FAIL:", err)
		os.Exit(1)
	}
	json.Unmarshal(b, &cfg)

	conn, err := dial(cfg.Gateway.Port, cfg.Gateway.Auth.Token)
	if err != nil {
		fmt.Println("DIAL FAIL:", err)
		os.Exit(1)
	}
	defer conn.Close()

	code, res := call(conn, "l1", "conversations.list", map[string]any{"agentId": "main", "limit": 8})
	fmt.Println("conversations.list ->", code, clip(res, 700))

	code, res = call(conn, "s1", "conversations.send", map[string]any{"agentId": "main", "conversationRef": "c_probe", "message": "x"})
	fmt.Println("conversations.send(bad ref) ->", code, clip(res, 200))
	fmt.Println("SPIKE5 DONE")
}

func dial(port int, token string) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://127.0.0.1:%d", port), nil)
	if err != nil {
		return nil, err
	}
	conn.SetReadDeadline(time.Now().Add(25 * time.Second))
	if _, _, err := conn.ReadMessage(); err != nil {
		return nil, err
	}
	connect := map[string]any{"type": "req", "id": "c1", "method": "connect",
		"params": map[string]any{"minProtocol": 4, "maxProtocol": 4,
			"client": map[string]any{"id": "gateway-client", "version": "0.0.1", "platform": "windows", "mode": "backend"},
			"role":   "operator",
			"scopes": []string{"operator.read", "operator.write", "operator.admin"},
			"auth":   map[string]any{"token": token}}}
	if err := conn.WriteJSON(connect); err != nil {
		return nil, err
	}
	for {
		var f map[string]any
		if err := conn.ReadJSON(&f); err != nil {
			return nil, err
		}
		if f["type"] == "res" && f["id"] == "c1" {
			if ok, _ := f["ok"].(bool); ok {
				return conn, nil
			}
			return nil, fmt.Errorf("connect rejected: %v", f["error"])
		}
	}
}

func call(conn *websocket.Conn, id, method string, params map[string]any) (string, string) {
	if err := conn.WriteJSON(map[string]any{"type": "req", "id": id, "method": method, "params": params}); err != nil {
		return "WRITE_ERR", err.Error()
	}
	for {
		var f map[string]any
		if err := conn.ReadJSON(&f); err != nil {
			return "READ_ERR", err.Error()
		}
		if f["type"] == "res" && f["id"] == id {
			if ok, _ := f["ok"].(bool); ok {
				return "OK", fmt.Sprint(f["payload"])
			}
			return "ERR", fmt.Sprint(f["error"])
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
