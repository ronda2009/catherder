// 网关 WebSocket 客户端：派发(sessions.send)与转发(conversations.send)都走这条。
// （sessions_send 在默认硬拒绝表，message 回 404），而 WS 这条路全通（cmd/spike2~5）。
// 握手三要点：
//
// client.id 必须是 "gateway-client"、mode 必须是 "backend"（别的值直接被枚举拒绝）；
// 环回 + 共享 token 可不带设备签名；
// 不报 scopes 只有隐式读权限 —— sessions.send 要 operator.write，conversations.send 要 operator.admin。
package openclaw

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn *websocket.Conn
	mu   sync.Mutex // Call 串行化：一条连接同一时刻只挂一个请求
	n    int
}

func Dial(cfg *Config) (*Client, error) {
	conn, _, err := websocket.DefaultDialer.Dial(cfg.WSDialURL(), nil)
	if err != nil {
		return nil, fmt.Errorf("连网关失败: %w", err)
	}
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	if _, _, err := conn.ReadMessage(); err != nil { // 预连接挑战
		conn.Close()
		return nil, fmt.Errorf("等挑战失败: %w", err)
	}
	connect := map[string]any{"type": "req", "id": "c1", "method": "connect",
		"params": map[string]any{"minProtocol": 4, "maxProtocol": 4,
			"client": map[string]any{"id": "gateway-client", "version": "0.1.0", "platform": "windows", "mode": "backend"},
			"role":   "operator",
			"scopes": []string{"operator.read", "operator.write", "operator.admin"},
			"auth":   map[string]any{"token": cfg.Gateway.Auth.Token}}}
	if err := conn.WriteJSON(connect); err != nil {
		conn.Close()
		return nil, err
	}
	for {
		var f map[string]any
		if err := conn.ReadJSON(&f); err != nil {
			conn.Close()
			return nil, fmt.Errorf("等 hello-ok 失败: %w", err)
		}
		if f["type"] == "res" && f["id"] == "c1" {
			if ok, _ := f["ok"].(bool); ok {
				return &Client{conn: conn}, nil
			}
			conn.Close()
			return nil, fmt.Errorf("握手被拒: %v", f["error"])
		}
	}
}

func (c *Client) Close() { c.conn.Close() }

// Call 发一个 RPC 并等它的应答，中间的 event 帧直接跳过。
func (c *Client) Call(method string, params map[string]any) (map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	id := fmt.Sprintf("r%d", c.n)
	if err := c.conn.WriteJSON(map[string]any{"type": "req", "id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	for {
		var f map[string]any
		if err := c.conn.ReadJSON(&f); err != nil {
			return nil, fmt.Errorf("等方法 %s 应答失败: %w", method, err)
		}
		if f["type"] == "res" && f["id"] == id {
			if ok, _ := f["ok"].(bool); ok {
				pl, _ := f["payload"].(map[string]any)
				return pl, nil
			}
			return nil, fmt.Errorf("方法 %s 报错: %v", method, f["error"])
		}
	}
}

// SendToWindow 把任务内容送进指定会话；timeoutSeconds 不给 = 不等回复（我们靠读库判定完成）。
func (c *Client) SendToWindow(sessionKey, message string) error {
	_, err := c.Call("sessions.send", map[string]any{"key": sessionKey, "message": message})
	return err
}

// SendToWindowWithImage 往窗口发一条文字 + 一张图（截图）。
// 网关 sessions.send 的 attachments 字段形状由源码确认：
// {mimeType, content(base64), fileName}，content 必须是 base64 字符串（也允许 data: 前缀）。
// 单图上限 6MB（网关 agent-hydration 上限），超了会被拒。
// SendToWindowWithFile 往窗口发一条带附件的消息（图片/pdf/文档都行，网关自己按 mimeType 处理）。
// 一个附件就包成单元素列表，交给 SendToWindowWithFiles。
func (c *Client) SendToWindowWithFile(sessionKey, message, mime string, data []byte, fileName string) error {
	return c.SendToWindowWithFiles(sessionKey, message, []Attach{{Mime: mime, Data: data, Name: fileName}})
}

type Attach struct {
	Mime string
	Data []byte
	Name string
}

// SendToWindowWithFiles 一条消息带多个附件（网关 sessions.send 的 attachments 本来就是数组）。
func (c *Client) SendToWindowWithFiles(sessionKey, message string, files []Attach) error {
	att := make([]any, 0, len(files))
	for _, f := range files {
		att = append(att, map[string]any{
			"mimeType": f.Mime, "content": base64.StdEncoding.EncodeToString(f.Data), "fileName": f.Name,
		})
	}
	_, err := c.Call("sessions.send", map[string]any{
		"key": sessionKey, "message": message, "attachments": att,
	})
	return err
}

// SendToConversation 往外部聊天地址（如用户飞书私聊）发一条，不等回复。
// operationId 必填，用它防重发；我们生成随机 hex 即可。
func (c *Client) SendToConversation(agentID, conversationRef, message string) error {
	b := make([]byte, 16)
	rand.Read(b)
	_, err := c.Call("conversations.send", map[string]any{
		"agentId": agentID, "conversationRef": conversationRef,
		"message": message, "operationId": hex.EncodeToString(b),
	})
	return err
}

// FindConversation 按通道+目标找 conversationRef。
// 找不到返回空串，调用方决定降级。
func (c *Client) FindConversation(agentID, channel, target string) (string, error) {
	pl, err := c.Call("conversations.list", map[string]any{"agentId": agentID, "limit": 50})
	if err != nil {
		return "", err
	}
	list, _ := pl["conversations"].([]any)
	for _, it := range list {
		m, _ := it.(map[string]any)
		ch, _ := m["channel"].(string)
		tg, _ := m["target"].(string)
		ref, _ := m["conversationRef"].(string)
		if ch != channel {
			continue
		}
		if target != "" && tg != target {
			continue
		}
		if ref != "" {
			return ref, nil
		}
	}
	return "", nil
}

var _ = json.Marshal
