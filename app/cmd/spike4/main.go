// spike4: 转发到聊天通道走哪条路。
// 已知：HTTP /tools/invoke 的 message 工具 404（被策略拒），sessions_send 也在 HTTP 硬拒绝表。
// 派发已验证走 WS sessions.send。转发候选逐个探针（都传坏参数，不真发消息）：
//
// WS: message.send / messages.send / conversations.send / chat.send(坏会话键)
// HTTP: conversations_send / message(带 x-openclaw-message-channel 头)
//
// 判读：方法不存在=明确报错；参数错/会话不存在=方法活着。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

type cfgT struct {
	Gateway struct {
		Port int                    `json:"port"`
		Auth struct{ Token string } `json:"auth"`
	} `json:"gateway"`
}

func main() {
	cfgPath := flag.String("config", `C:\Users\Administrator\.openclaw\openclaw.json`, "")
	flag.Parse()
	var cfg cfgT
	b, err := os.ReadFile(*cfgPath)
	if err != nil {
		fmt.Println("CFG FAIL:", err)
		os.Exit(1)
	}
	json.Unmarshal(b, &cfg)
	port := cfg.Gateway.Port
	token := cfg.Gateway.Auth.Token

	// HTTP 候选
	for _, t := range []string{"conversations_send", "message"} {
		code, body := httpInvoke(port, token, t)
		fmt.Printf("HTTP %-18s -> %d %s\n", t, code, clip(body, 130))
	}

	// WS 候选：一次连接试多个方法
	conn, err := dial(port, token)
	if err != nil {
		fmt.Println("WS DIAL FAIL:", err)
		os.Exit(1)
	}
	defer conn.Close()
	type probe struct {
		method string
		params map[string]any
	}
	probes := []probe{
		{"message.send", map[string]any{"channel": "__probe__", "target": "__probe__", "message": "x"}},
		{"messages.send", map[string]any{"channel": "__probe__", "target": "__probe__", "message": "x"}},
		{"conversations.send", map[string]any{"conversationRef": "__probe__", "message": "x"}},
		{"chat.send", map[string]any{"sessionKey": "agent:main:dashboard:spike-nonexistent-0000", "message": "x"}},
	}
	for i, p := range probes {
		id := fmt.Sprintf("p%d", i)
		if err := conn.WriteJSON(map[string]any{"type": "req", "id": id, "method": p.method, "params": p.params}); err != nil {
			fmt.Println("WRITE ERR", p.method, err)
			return
		}
		code, res := readResp(conn, id)
		fmt.Printf("WS %-18s -> %s %s\n", p.method, code, clip(res, 150))
	}
	fmt.Println("SPIKE4 DONE")
}

func dial(port int, token string) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://127.0.0.1:%d", port), nil)
	if err != nil {
		return nil, err
	}
	conn.SetReadDeadline(time.Now().Add(25 * time.Second))
	if _, _, err := conn.ReadMessage(); err != nil { // challenge
		return nil, err
	}
	connect := map[string]any{"type": "req", "id": "c1", "method": "connect",
		"params": map[string]any{"minProtocol": 4, "maxProtocol": 4,
			"client": map[string]any{"id": "gateway-client", "version": "0.0.1", "platform": "windows", "mode": "backend"},
			// 上一轮实测：message.* 与 conversations.send 要 operator.admin，补进申请范围
			"role": "operator", "scopes": []string{"operator.read", "operator.write", "operator.admin"},
			"auth": map[string]any{"token": token}}}
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

func readResp(conn *websocket.Conn, id string) (string, string) {
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

func httpInvoke(port int, token, tool string) (int, string) {
	args := map[string]any{"action": "send", "channel": "__probe__", "target": "__probe__", "message": "x"}
	if tool == "conversations_send" {
		args = map[string]any{"conversationRef": "__probe__", "message": "x"}
	}
	payload, _ := json.Marshal(map[string]any{"tool": tool, "args": args})
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d/tools/invoke", port), jsonReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return -1, "ERR " + err.Error()
	}
	defer resp.Body.Close()
	bb, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
	return resp.StatusCode, string(bb)
}

func jsonReader(b []byte) io.Reader { return &byteReader{b: b} }

type byteReader struct {
	b []byte
	i int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
