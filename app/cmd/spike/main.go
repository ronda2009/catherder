// spike: M1a 尖测 —— Go 能否只读打开 OpenClaw 会话库（WAL 模式）并读到
// 调度需要的三样东西：窗口状态、待处理输入、最后一条回复正文。
// 这一步过了，整个软件才敢全押 Go；不过就走 Tauri / Node 后备。
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	_ "modernc.org/sqlite" // 纯 Go 驱动，免 CGO；OpenClaw 写库、我们只读，WAL 允许并发读
)

type sessionRow struct {
	Key     string  `json:"key"`
	Label   *string `json:"label"`
	Status  *string `json:"status"` // NULL = 正在跑（实测结论，）
	Model   *string `json:"model"`
	Updated int64   `json:"updated_at"`
}

func main() {
	dbPath := flag.String("db", `C:\Users\Administrator\.openclaw\agents\main\agent\openclaw-agent.sqlite`, "OpenClaw 会话库路径")
	flag.Parse()

	// mode=ro：只读，绝不写库，绝不碰 OpenClaw 的写锁
	dsn := "file:" + *dbPath + "?mode=ro&_pragma=busy_timeout(3000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		fmt.Println("OPEN FAIL:", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		fmt.Println("PING FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("OPEN OK (read-only)")

	// 1) 窗口状态分布：判完工的主判据
	rows, err := db.Query(`SELECT status, COUNT(*) FROM session_nodes GROUP BY status`)
	if err != nil {
		fmt.Println("STATUS QUERY FAIL:", err)
		os.Exit(1)
	}
	for rows.Next() {
		var st sql.NullString
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			continue
		}
		s := "NULL(running)"
		if st.Valid {
			s = st.String
		}
		fmt.Println("STATUS", s, "=", n)
	}
	rows.Close()

	// 2) 哪些窗口还堆着没处理的输入（判「后台没活」）
	rows, err = db.Query(`SELECT session_key, COUNT(*) FROM session_pending_inputs GROUP BY session_key ORDER BY 2 DESC LIMIT 5`)
	if err != nil {
		fmt.Println("PENDING QUERY FAIL (表可能不存在=没有堆队):", err)
	} else {
		n := 0
		for rows.Next() {
			var k string
			var c int
			if err := rows.Scan(&k, &c); err != nil {
				continue
			}
			n++
			fmt.Println("PENDING", k, "=", c)
		}
		if n == 0 {
			fmt.Println("PENDING: none")
		}
		rows.Close()
	}

	// 3) 取一个已结束窗口的最后一条事件，验证 event_json 能当明文读（转发飞书要用）
	var sid string
	err = db.QueryRow(`SELECT current_session_id FROM session_nodes
		WHERE status='done' AND session_key LIKE 'agent:main:dashboard:%'
		ORDER BY updated_at DESC LIMIT 1`).Scan(&sid)
	if err != nil {
		fmt.Println("NO DONE DASHBOARD SESSION:", err)
		return
	}
	var evJSON sql.NullString
	err = db.QueryRow(`SELECT event_json FROM transcript_events
		WHERE session_id = ? ORDER BY seq DESC LIMIT 3`, sid).Scan(&evJSON)
	if err != nil {
		fmt.Println("TRANSCRIPT QUERY FAIL:", err)
		return
	}
	if !evJSON.Valid {
		fmt.Println("LAST EVENT: event_json 为空（可能是压缩列 event_zstd，Go 侧同样要后备）")
	} else {
		var ev map[string]any
		if err := json.Unmarshal([]byte(evJSON.String), &ev); err != nil {
			fmt.Println("EVENT JSON PARSE FAIL:", err)
			return
		}
		t, _ := ev["type"].(string)
		fmt.Printf("LAST EVENT type=%s bytes=%d 前 120 字: %s\n", t, len(evJSON.String), clip(evJSON.String, 120))
	}
	fmt.Println("SPIKE PASS")
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
