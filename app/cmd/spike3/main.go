// spike3: 转发用的两件事 ——
//  1. message 工具能不能走 HTTP /tools/invoke（它不在硬拒绝表里，预期可用；
//     故意传坏参数：400=工具活着，404=被拒，避免真发一条消息打扰用户）
//  2. transcript_events 里 message 事件的正文到底在哪个字段（转发要截取它）
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	cfgPath := flag.String("config", `C:\Users\Administrator\.openclaw\openclaw.json`, "")
	dbPath := flag.String("db", `C:\Users\Administrator\.openclaw\agents\main\agent\openclaw-agent.sqlite`, "")
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

	// 1) message 工具可用性探针（坏参数）
	body := `{"tool":"message","args":{"action":"send","channel":"__probe__","target":"__probe__","message":"x"}}`
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d/tools/invoke", cfg.Gateway.Port),
		newReader(body))
	req.Header.Set("Authorization", "Bearer "+cfg.Gateway.Auth.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 20e9}).Do(req)
	if err != nil {
		fmt.Println("HTTP ERR:", err)
	} else {
		defer resp.Body.Close()
		bb, _ := io.ReadAll(io.LimitReader(resp.Body, 600))
		fmt.Println("HTTP message(probe) ->", resp.StatusCode, string(bb))
	}

	// 2) 最后一条 message 事件的字段形状
	db, err := sql.Open("sqlite", "file:"+*dbPath+"?mode=ro&_pragma=busy_timeout(3000)")
	if err != nil {
		fmt.Println("DB FAIL:", err)
		os.Exit(1)
	}
	defer db.Close()
	var sid string
	db.QueryRow(`SELECT current_session_id FROM session_nodes WHERE status='done'
		AND session_key LIKE 'agent:main:dashboard:%' ORDER BY updated_at DESC LIMIT 1`).Scan(&sid)
	rows, err := db.Query(`SELECT seq, event_json FROM transcript_events WHERE session_id=?
		AND event_json IS NOT NULL ORDER BY seq DESC LIMIT 6`, sid)
	if err != nil {
		fmt.Println("TX FAIL:", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var seq int
		var ej sql.NullString
		rows.Scan(&seq, &ej)
		if !ej.Valid {
			continue
		}
		var ev map[string]any
		if json.Unmarshal([]byte(ej.String), &ev) != nil {
			continue
		}
		t, _ := ev["type"].(string)
		if t != "message" {
			fmt.Println("SEQ", seq, "type=", t)
			continue
		}
		keys := make([]string, 0, 8)
		for k := range ev {
			keys = append(keys, k)
		}
		fmt.Println("SEQ", seq, "message keys:", keys)
		bb, _ := json.Marshal(ev)
		fmt.Println("  SHAPE:", clip(string(bb), 400))
		break
	}
	fmt.Println("SPIKE3 DONE")
}

func newReader(s string) io.Reader { return &stringReader{s: s} }

type stringReader struct {
	s string
	i int
}

func (r *stringReader) Read(p []byte) (int, error) {
	if r.i >= len(r.s) {
		return 0, io.EOF
	}
	n := copy(p, r.s[r.i:])
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
