// 只读打开 OpenClaw 会话库（mode=ro，绝不写、绝不抢写锁）。
// 判定任务完成要用的三样东西都从这里取：窗口状态、待处理输入、最后一条回复正文。
// 实测依据见 cmd/spike 与《接口实测记录.md》。
package openclaw

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // 纯 Go 驱动免 CGO；WAL 允许我们并发只读
)

type Window struct {
	Key       string
	ID        string // current_session_id，查对话流水用
	Label     string
	Status    string // 空 = 正在跑（实测：库里 NULL 表示运行中）
	UpdatedAt int64
	CreatedAt int64 // 建窗时间：定「固定顺序」用，不随活动变
	PinnedAt  int64 // 用户在 OpenClaw 里置顶过的窗口排最前
	Archived  bool  // 归档的窗口不进清单（和 OpenClaw 界面一致）
	Model     string
	Provider  string
	Pending   int // 还堆着没处理的输入条数
}

// Idle 判「窗口这一侧已完成」：状态已结束。是否再等延时复核由引擎决定（定稿 §4）。
func (w *Window) Idle() bool {
	return w.Status == "done" || w.Status == "failed" || w.Status == "killed"
}

// Schedulable 排除心跳/定时任务/子代理/系统内部造出来的窗口（实测本机 42 个里这类占 26 个）。
// internal-session-effects、selfcheck 这类是 OpenClaw 自己开的内部会话（实测漏过两个），
// 派活给它们没意义，还会把清单挤乱。
func (w *Window) Schedulable() bool {
	k := w.Key
	if !strings.HasPrefix(k, "agent:") {
		return false
	}
	for _, seg := range strings.Split(k, ":") {
		switch {
		case seg == "heartbeat", seg == "cron", seg == "subagent":
			return false
		case strings.Contains(seg, "internal-session"), strings.HasPrefix(seg, "selfcheck"):
			return false
		}
	}
	return true
}

type DB struct{ sql *sql.DB }

func OpenDB(path string) (*DB, error) {
	// busy_timeout 让我们撞上写事务时多等一会儿而不是直接报错
	dsn := "file:" + path + "?mode=ro&_pragma=busy_timeout(3000)"
	s, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := s.Ping(); err != nil {
		s.Close()
		return nil, err
	}
	return &DB{sql: s}, nil
}

func (d *DB) Close() { d.sql.Close() }

type entryShape struct {
	Label         string `json:"label"`
	DisplayName   string `json:"displayName"`
	Model         string `json:"model"`
	ModelProvider string `json:"modelProvider"`
}

func (d *DB) Windows() ([]Window, error) {
	// 列里的 pinned_at / created_at / archived_at 是后加的一列（实测 session_nodes 有这些列）。
	// 排序不能按 updated_at：那个字段每有动静就变，界面会上下串行（用户 2026-10-24 反馈）。
	// 这里只取数据，固定顺序由引擎排（置顶优先 → 建窗时间 → 键名）。
	rows, err := d.sql.Query(`SELECT session_key, current_session_id, status, updated_at, entry_json,
		COALESCE(created_at, 0), COALESCE(pinned_at, 0), archived_at IS NOT NULL
		FROM session_nodes ORDER BY updated_at DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pending, err := d.pendingBySession()
	if err != nil {
		return nil, err
	}

	var out []Window
	for rows.Next() {
		var (
			w      Window
			ej     sql.NullString
			status sql.NullString
			curID  string
		)
		if err := rows.Scan(&w.Key, &curID, &status, &w.UpdatedAt, &ej, &w.CreatedAt, &w.PinnedAt, &w.Archived); err != nil {
			return nil, err
		}
		w.ID = curID
		w.Status = status.String // NULL → ""，即“正在跑”
		if ej.Valid {
			var e entryShape
			if json.Unmarshal([]byte(ej.String), &e) == nil {
				w.Label = firstNonEmpty(e.Label, e.DisplayName)
				w.Model = e.Model
				w.Provider = e.ModelProvider
			}
		}
		w.Pending = pending[w.Key]
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	OrderWindows(out)
	return out, nil
}

// OrderWindows 定「固定顺序」：置顶优先 → 建窗时间（早的在前）→ 窗口键。
// 不能按 updated_at 排：那个字段每有动静（勾选、刷新、派活、收到消息）就变，
// 清单就会上下串行（用户 2026-10-24 反馈「勾选窗口会上下串行、点过去也变一下、下拉菜单也变」）。
// 建窗时间和窗口键都不随活动变 → 只要窗口没新建/删除，位置就钉死。
// 置顶跟 OpenClaw 控制界面一致：用户在那边置顶了，这边也跟着排最前。
func OrderWindows(list []Window) {
	sort.SliceStable(list, func(i, j int) bool {
		if (list[i].PinnedAt > 0) != (list[j].PinnedAt > 0) {
			return list[i].PinnedAt > 0
		}
		if list[i].CreatedAt != list[j].CreatedAt {
			if list[i].CreatedAt == 0 {
				return false // 没建窗时间的退到后面，但仍稳定
			}
			if list[j].CreatedAt == 0 {
				return true
			}
			return list[i].CreatedAt < list[j].CreatedAt
		}
		return list[i].Key < list[j].Key
	})
}

// pendingBySession：consumed_event_id 为空 = 这条输入还没被回合消费（表结构实测）。
// 但**还得排掉已作废的**：state 为 cancelled / interrupted 的那批永远不会再被消费
// （本机实测：未消费的 7 条全是这两种，最早一条已挂 10 天）。
// 不排掉就会把窗口永远当成「还有话没处理」→ 排队任务永远派不出去（用户 2026-10-24 反馈）。
func (d *DB) pendingBySession() (map[string]int, error) {
	rows, err := d.sql.Query(`SELECT session_key, COUNT(*) FROM session_pending_inputs
		WHERE consumed_event_id IS NULL
		  AND COALESCE(state,'') NOT IN ('cancelled','interrupted','failed')
		GROUP BY session_key`)
	if err != nil {
		// 老版本库可能没这张表：没有堆队可视为 0
		if strings.Contains(err.Error(), "no such table") {
			return map[string]int{}, nil
		}
		return nil, err
	}
	defer rows.Close()
	m := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		m[k] = n
	}
	return m, rows.Err()
}

type eventShape struct {
	Type    string `json:"type"`
	Message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// textFromContent 兼容 content 是字符串或块数组两种形状。
func textFromContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		var parts []string
		for _, b := range blocks {
			if b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// LastAssistantText 取该窗口最后一条助手回复（转发飞书用）。
// 只读 event_json 明文列；压缩列 event_zstd 本机实测 Node/Go 自带解压都解不开，先不管。
func (d *DB) LastAssistantText(sessionID string) (string, error) {
	rows, err := d.sql.Query(`SELECT event_json FROM transcript_events
		WHERE session_id = ? AND event_json IS NOT NULL ORDER BY seq DESC LIMIT 40`, sessionID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var ej string
		if err := rows.Scan(&ej); err != nil {
			continue
		}
		var ev eventShape
		if json.Unmarshal([]byte(ej), &ev) != nil {
			continue
		}
		if ev.Type == "message" && ev.Message.Role == "assistant" {
			if t := textFromContent(ev.Message.Content); strings.TrimSpace(t) != "" {
				return t, nil
			}
		}
	}
	return "", nil
}

// ContainsSince 在派发时刻之后的助手回复里找完成尾巴（加强证据，不是唯一判据）。
func (d *DB) ContainsSince(sessionID string, sinceMs int64, needle string) (bool, error) {
	rows, err := d.sql.Query(`SELECT event_json FROM transcript_events
		WHERE session_id = ? AND event_json IS NOT NULL AND created_at > ?
		ORDER BY seq DESC LIMIT 60`, sessionID, sinceMs)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var ej string
		if err := rows.Scan(&ej); err != nil {
			continue
		}
		var ev eventShape
		if json.Unmarshal([]byte(ej), &ev) != nil {
			continue
		}
		if ev.Type == "message" && ev.Message.Role == "assistant" {
			if strings.Contains(textFromContent(ev.Message.Content), needle) {
				return true, nil
			}
		}
	}
	return false, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

var ErrNoWindow = errors.New("window not found")
