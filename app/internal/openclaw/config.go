// 读 OpenClaw 自己的配置（网关端口/鉴权/服务商地址/已启用通道）。
// 发布给外人用时必须自动探测，不能写死本机值 —— 见设计。
package openclaw

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ProviderEntry 服务商条目（导出给测试与界面用）。
type ProviderEntry struct {
	BaseURL string `json:"baseUrl"`
}

type Config struct {
	Path    string `json:"-"`
	Gateway struct {
		Port int `json:"port"`
		Auth struct {
			Mode  string `json:"mode"`
			Token string `json:"token"`
		} `json:"auth"`
	} `json:"gateway"`
	Channels map[string]json.RawMessage `json:"channels"`
	Models   struct {
		Providers map[string]ProviderEntry `json:"providers"`
	} `json:"models"`
}

// DefaultConfigPath 按 home 推导，换机器不用改。
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".openclaw/openclaw.json"
	}
	return filepath.Join(home, ".openclaw", "openclaw.json")
}

// DefaultDBPath 同理：会话库位置随 home 走。
func DefaultDBPath(agent string) string {
	if agent == "" {
		agent = "main"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".openclaw/agents/main/agent/openclaw-agent.sqlite"
	}
	return filepath.Join(home, ".openclaw", "agents", agent, "agent", "openclaw-agent.sqlite")
}

func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := &Config{Path: path}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	if c.Gateway.Port == 0 {
		c.Gateway.Port = 18789 // OpenClaw 默认端口
	}
	return c, nil
}

func (c *Config) WSDialURL() string {
	return fmt.Sprintf("ws://127.0.0.1:%d", c.Gateway.Port)
}

// EnabledChannels 返回配置里出现过的通道名（通知通道探测用，a）。
func (c *Config) EnabledChannels() []string {
	out := make([]string, 0, len(c.Channels))
	for k := range c.Channels {
		out = append(out, k)
	}
	return out
}

// ProviderIsLocal 用服务商地址判断本地/云端。
func (c *Config) ProviderIsLocal(name string) (bool, string) {
	p, ok := c.Models.Providers[name]
	if !ok || p.BaseURL == "" {
		return false, ""
	}
	u, err := url.Parse(p.BaseURL)
	if err != nil {
		return false, p.BaseURL
	}
	h := strings.ToLower(u.Hostname())
	return h == "127.0.0.1" || h == "localhost" || h == "::1", u.Host
}
