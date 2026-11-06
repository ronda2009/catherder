// 派工台自己的 SQLite：任务记录、窗口勾选、设置、日志。
// 与 OpenClaw 的库完全分开——我们只读它的，写只写自己的。
// 编号用自增 id 显示成 T-001：日期不进编号（太长、难念、同日会撞）。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// json 标签是给界面用的：不加标签 Go 会把字段名原样输出成 ID/Body/TargetKey
// 前端按小驼峰取就全是 undefined。
type Task struct {
	ID           int      `json:"id"`
	Body         string   `json:"body"`
	TargetKey    string   `json:"targetKey"`
	TargetLabel  string   `json:"targetLabel"`
	State        string   `json:"state"` // queued|dispatched|done|done_unacked|error|canceled|paused
	CreatedAt    int64    `json:"createdAt"`
	DispatchedAt int64    `json:"dispatchedAt"`
	DoneAt       int64    `json:"doneAt"`
	Ack          bool     `json:"ack"`
	Tries        int      `json:"tries"`
	Prio         float64  `json:"prio"` // 拖动调序用；默认跟 id 一致
	Note         string   `json:"note"`
	IdleSince    int64    `json:"idleSince"`   // 目标窗口首次被观察到空闲的时刻；复核计时用（落盘，重启不丢）
	NotifiedAt   int64    `json:"notifiedAt"`  // 结果已送达通知通道的时刻；0 = 还没送达
	ImagePath    string   `json:"imagePath"`   // 随任务一起发的图片（截图）在本机的路径；空 = 没图
	AttachPaths  []string `json:"attachPaths"` // 随任务一起发的全部附件；空 = 没附件
	Editing      bool     `json:"editing"`     // 正在队列里就地改：改完按「改好了」才参与派发
}

type Store struct{ db *sql.DB }

func DefaultPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	dir := filepath.Join(base, "catherder")
	os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "catherder.sqlite")
}

func Open(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout(3000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	if err := initSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// RecoverEditing 界面版启动时调：把上次没按「改好了」就关掉的条推回排队。
// 没存的内容本来就已经丢了，卡在「正在改」还会挡住自动关机。
// 只在界面版启动时做，不能放进 Open：命令行每条命令都开一次库
// 那样用户正在界面里改着时，随手跑一条 `queue` 就会把这条打回排队。
func (s *Store) RecoverEditing() {
	_, _ = s.db.Exec(`UPDATE tasks SET state='queued', editing_at=0 WHERE state='editing'`)
}

func (s *Store) Close() { s.db.Close() }

func initSchema(db *sql.DB) error {
	stmts := `
CREATE TABLE IF NOT EXISTS tasks(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  body TEXT NOT NULL,
  target_key TEXT NOT NULL,
  target_label TEXT,
  state TEXT NOT NULL DEFAULT 'queued',
  created_at INTEGER, dispatched_at INTEGER, done_at INTEGER,
  ack INTEGER DEFAULT 0, tries INTEGER DEFAULT 0,
  prio REAL, note TEXT, idle_since INTEGER, notified_at INTEGER);
CREATE TABLE IF NOT EXISTS windows(
  key TEXT PRIMARY KEY, label TEXT,
  included INTEGER DEFAULT 0,
  last_seen INTEGER, last_status TEXT, model TEXT, provider TEXT);
CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY, value TEXT);
CREATE TABLE IF NOT EXISTS events(id INTEGER PRIMARY KEY AUTOINCREMENT, at INTEGER, task_id INTEGER, kind TEXT, detail TEXT);
`
	if _, err := db.Exec(stmts); err != nil {
		return err
	}
	// 老库补列：复核计时必须落盘——命令行版一次只跑一轮，存在内存里就等于每次从头计时、永远复核不完
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('tasks') WHERE name='idle_since'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN idle_since INTEGER`); err != nil {
			return err
		}
	}
	// 老库补列：「已完成」区要区分结果到底送达了通知通道没有
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('tasks') WHERE name='notified_at'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN notified_at INTEGER`); err != nil {
			return err
		}
	}
	// 老库补列：截图（图片）跟任务一起发，路径要落盘——重启后重发还能带上图
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('tasks') WHERE name='image_path'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN image_path TEXT`); err != nil {
			return err
		}
	}
	// 老库补列：一条任务可带多个附件，换行存进这一列
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('tasks') WHERE name='attach_paths'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN attach_paths TEXT`); err != nil {
			return err
		}
	}
	// 老库补列：「正在改」的入档时刻——排队区把它排在最前、显示「正在编辑」
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('tasks') WHERE name='editing_at'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN editing_at INTEGER`); err != nil {
			return err
		}
	}
	return nil
}

// ---- 任务 ----

func (s *Store) AddTask(body, targetKey, targetLabel string) (int, error) {
	// 先插再把自己的 id 当 prio：VALUES 里不能拿聚合函数读 id。
	// 默认 prio=id 就等于「按提交顺序」，拖动调序时才改 prio。
	res, err := s.db.Exec(`INSERT INTO tasks(body,target_key,target_label,state,created_at)
		VALUES(?,?,?,'queued',strftime('%s','now')*1000)`, body, targetKey, targetLabel)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if _, err := s.db.Exec(`UPDATE tasks SET prio=? WHERE id=?`, float64(id), id); err != nil {
		return 0, err
	}
	s.logEvent(int(id), "add", "加入队列 → "+targetLabel)
	return int(id), nil
}

func taskIDFromPrio(p float64) int { return int(p) }

// Queue 返回未派发任务（排队中/暂停/正在改），按提交顺序（prio 跟 id 走，拖动调序改 prio）。
// 「正在改」的不抢位也不提前：改完按「改好了」回到原位，派发时先跳过它。
func (s *Store) Queue() ([]Task, error) {
	return s.queryTasks(`WHERE state IN ('queued','paused','editing') ORDER BY prio, id`)
}

// Dispatched 返回已派发未收尾的任务。
func (s *Store) Dispatched() ([]Task, error) {
	return s.queryTasks(`WHERE state='dispatched' ORDER BY dispatched_at`)
}

// DispatchedSince 给界面用：本轮开过机以来派出去的任务全部留着
// 已完成的也不抹掉，直到程序关掉；重开就是新的一轮（since 传本次启动时刻）。
// 引擎算并发仍用 Dispatched()（只数真正在跑的），两者不能混。
// DispatchedSince 给界面「已派发待回复 + 已完成」：
//
// 仍在执行的（dispatched）全留 —— 不管哪轮派出去的，它还没完就该看得见；
// 已完成的只留本轮完成的 —— 按 done_at 判，不按 dispatched_at。
//
// 「上轮派发、本轮才完成」的那条会在判定完成那一刻从界面上直接消失（跳没）
// 用户看不到它变绿，还以为程序把任务弄丢了。
func (s *Store) DispatchedSince(since int64) ([]Task, error) {
	return s.queryTasks(`WHERE state='dispatched'
		OR (state IN ('done','done_unacked') AND IFNULL(done_at, dispatched_at) >= ?)
		ORDER BY dispatched_at`, since)
}

func (s *Store) queryTasks(where string, args ...any) ([]Task, error) {
	rows, err := s.db.Query(`SELECT id,body,target_key,IFNULL(target_label,''),state,
		IFNULL(created_at,0),IFNULL(dispatched_at,0),IFNULL(done_at,0),IFNULL(ack,0),
		IFNULL(tries,0),IFNULL(prio,0),IFNULL(note,''),IFNULL(idle_since,0),IFNULL(notified_at,0),IFNULL(image_path,''),IFNULL(attach_paths,''),IFNULL(editing_at,0) FROM tasks `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// 空结果也要返空数组，不能留 nil：nil 切片到界面就是 null
	// 界面取 .length 会报「Cannot read properties of null (reading 'length')」
	out := []Task{}
	for rows.Next() {
		var t Task
		var ack int
		var attachRaw string
		var editingAt int64
		if err := rows.Scan(&t.ID, &t.Body, &t.TargetKey, &t.TargetLabel, &t.State,
			&t.CreatedAt, &t.DispatchedAt, &t.DoneAt, &ack, &t.Tries, &t.Prio, &t.Note, &t.IdleSince, &t.NotifiedAt, &t.ImagePath, &attachRaw, &editingAt); err != nil {
			return nil, err
		}
		t.Ack = ack == 1
		t.AttachPaths = splitPaths(attachRaw, t.ImagePath)
		t.Editing = editingAt > 0
		out = append(out, t)
	}
	return out, rows.Err()
}

// MarkNotified 记下「结果已送达通知通道」。界面据此把任务从「已派发待回复」挪进「已完成」。
func (s *Store) MarkNotified(id int) error {
	_, err := s.db.Exec(`UPDATE tasks SET notified_at=strftime('%s','now')*1000 WHERE id=? AND notified_at IS NULL`, id)
	return err
}

// SetImagePath 记下这条任务带的图片（截图）路径。空串表示不带。
func (s *Store) SetImagePath(id int, path string) error {
	_, err := s.db.Exec(`UPDATE tasks SET image_path=? WHERE id=?`, path, id)
	return err
}

// SetAttachPaths 记下这条任务带的全部附件（换行存进 attach_paths）。
func (s *Store) SetAttachPaths(id int, paths []string) error {
	_, err := s.db.Exec(`UPDATE tasks SET attach_paths=? WHERE id=?`, strings.Join(paths, "\n"), id)
	return err
}

// splitPaths 把 attach_paths 拆成切片；老任务只有 image_path 时把它当成唯一附件。
func splitPaths(raw, legacy string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, p := range strings.Split(raw, "\n") {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	if len(out) == 0 && legacy != "" {
		out = append(out, legacy)
	}
	return out
}

func (s *Store) MarkDispatched(id int) error {
	_, err := s.db.Exec(`UPDATE tasks SET state='dispatched', dispatched_at=strftime('%s','now')*1000,
		tries=tries+1 WHERE id=?`, id)
	if err == nil {
		s.logEvent(id, "dispatch", "")
	}
	return err
}

// MarkDone：ack=false 时状态记 done_unacked（卡片标灰但仍推进，）。
func (s *Store) MarkDone(id int, ack bool) error {
	state := "done"
	if !ack {
		state = "done_unacked"
	}
	_, err := s.db.Exec(`UPDATE tasks SET state=?, done_at=strftime('%s','now')*1000, ack=? WHERE id=?`,
		state, boolInt(ack), id)
	if err == nil {
		s.logEvent(id, "done", "ack="+state)
	}
	return err
}

// MarkIdle 记下「目标窗口从这一刻起空闲」；已有值不覆盖——复核要的就是首次时刻。
func (s *Store) MarkIdle(id int, ms int64) error {
	_, err := s.db.Exec(`UPDATE tasks SET idle_since=? WHERE id=? AND IFNULL(idle_since,0)=0`, ms, id)
	return err
}

// ClearIdle：窗口又忙了，复核重新计时。
func (s *Store) ClearIdle(id int) error {
	_, err := s.db.Exec(`UPDATE tasks SET idle_since=NULL WHERE id=? AND IFNULL(idle_since,0)<>0`, id)
	return err
}

func (s *Store) MarkError(id int, note string) error {
	_, err := s.db.Exec(`UPDATE tasks SET state='error', note=?, done_at=strftime('%s','now')*1000 WHERE id=?`, note, id)
	if err == nil {
		s.logEvent(id, "error", note)
	}
	return err
}

// Counts 按状态数任务，给「队列里全标异常了就停下来」这类总览用。
func (s *Store) Counts() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT state, COUNT(*) FROM tasks GROUP BY state`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		out[st] = n
	}
	return out, rows.Err()
}

// RecentErrors 取待处理的异常任务（含已取消摆着的），给界面异常区与命令行 errors 用。
// 未处理的排前面，已取消的排后面——界面要先把要管的露出来。
func (s *Store) RecentErrors(limit int) ([]Task, error) {
	return s.queryTasks(`WHERE state IN ('error','dismissed') ORDER BY (state='dismissed'), done_at DESC LIMIT ?`, limit)
}

// AllTasks 给导出 CSV 用：不分状态，按时间倒序。
func (s *Store) AllTasks(limit int) ([]Task, error) {
	return s.queryTasks(`ORDER BY id DESC LIMIT ?`, limit)
}

// RetryTask 把标异常（或已取消摆着）的任务重新放回队列（用户修好模型后点「重试这条」）。
func (s *Store) RetryTask(id int) error {
	_, err := s.db.Exec(`UPDATE tasks SET state='queued', note='', tries=0, done_at=0, idle_since=0 WHERE id=? AND state IN ('error','dismissed')`, id)
	if err == nil {
		s.logEvent(id, "retry", "重新排队")
	}
	return err
}

// Dismiss 把一条异常任务「取消」：不再派发、也不算删除，就摆着（state='dismissed'）。
// 界面不再拿它弹横幅、它也不拦自动关机；库里记录与内容都留着，随时还能重试或删除。
// 最后一条异常被取消/处理完 → 顺手清掉「已停止派发」标记，横幅当场复位（不等下一轮检查）。
func (s *Store) Dismiss(id int) error {
	res, err := s.db.Exec(`UPDATE tasks SET state='dismissed' WHERE id=? AND state='error'`, id)
	if err != nil {
		return err
	}
	// 没改到行（这条早就不在异常列表里）就别写日志也别清停机标记，否则库里会多一条假记录。
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("T-%03d 不在异常列表里，取消不了", id)
	}
	s.logEvent(id, "dismiss", "已取消：不再派发，也不删除，留在异常任务里")
	return s.clearHaltIfNoError()
}

// DismissAllErrors 横幅上的「取消」：把当前这批异常一次性都取消摆着，并复位「已停止派发」。
func (s *Store) DismissAllErrors() error {
	if _, err := s.db.Exec(`UPDATE tasks SET state='dismissed' WHERE state='error'`); err != nil {
		return err
	}
	s.Log("dismiss", "批量取消异常任务（不再派发，也不删除）")
	return s.clearHaltIfNoError()
}

// clearHaltIfNoError 没有待处理的异常了就把停机标记抹掉（有排队任务时引擎也会自己抹）。
func (s *Store) clearHaltIfNoError() error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE state='error'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return s.ClearHalt()
	}
	return nil
}

// ClearHalt 异常处理完成 → 清除「全部异常」停机标记。
func (s *Store) ClearHalt() error { return s.SetSetting("halt", "") }

func (s *Store) Cancel(id int) error {
	// 标异常（含已取消摆着）的任务也得能撤（失败分诊后用户可能直接丢掉这条）；正在改的也能撤。
	_, err := s.db.Exec(`UPDATE tasks SET state='canceled', done_at=strftime('%s','now')*1000 WHERE id=? AND state IN ('queued','error','dismissed','paused','editing')`, id)
	if err == nil {
		s.logEvent(id, "cancel", "")
	}
	return err
}

// StartEditing 把一条排队任务转成「正在改」：内容/发送窗口/附件随便改，改完按「改好了」才参与派发。
// 目的：用户在队列里点「编辑」不该把上方新建任务里写的长文冲掉
// 而且改的时候引擎不能把它派出去。
func (s *Store) StartEditing(id int) error {
	res, err := s.db.Exec(`UPDATE tasks SET state='editing', editing_at=strftime('%s','now')*1000 WHERE id=? AND state='queued'`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("这条已派发或已不在排队，改不了")
	}
	s.logEvent(id, "edit", "开始就地修改（先不派发）")
	return nil
}

// FinishEditing 改完：存正文与发送窗口，回到排队（prio 没动，所以回到原来的位置）。
// body 为空表示不改内容（只改了发送窗口）。
func (s *Store) FinishEditing(id int, body, targetKey, targetLabel string) error {
	q, err := s.Queue()
	if err != nil {
		return err
	}
	found := false
	for _, t := range q {
		if t.ID == id {
			found = true
			break
		}
	}
	if !found { // 不在排队列表里（已派发/已删）：不覆盖现有内容，只把状态推回排队
		if _, err := s.db.Exec(`UPDATE tasks SET state='queued', editing_at=0 WHERE id=? AND state='editing'`, id); err == nil {
			s.logEvent(id, "edit", "改完，重新排队")
		}
		return nil
	}
	if body == "" {
		_, err = s.db.Exec(`UPDATE tasks SET state='queued', editing_at=0, target_key=?, target_label=? WHERE id=? AND state='editing'`, targetKey, targetLabel, id)
	} else {
		_, err = s.db.Exec(`UPDATE tasks SET state='queued', editing_at=0, body=?, target_key=?, target_label=? WHERE id=? AND state='editing'`, body, targetKey, targetLabel, id)
	}
	if err != nil {
		return err
	}
	s.logEvent(id, "edit", "改完，重新排队")
	return nil
}

// CancelEditing 放弃这次修改：内容一个字都没写进库（按「改好了」才写），所以只把状态推回排队。
func (s *Store) CancelEditing(id int) error {
	_, err := s.db.Exec(`UPDATE tasks SET state='queued', editing_at=0 WHERE id=? AND state='editing'`, id)
	if err == nil {
		s.logEvent(id, "edit", "放弃修改，按原样排队")
	}
	return err
}

// Move 把任务 id 挪到 beforeID 前面（beforeID=0 挪队首，-1 挪队尾）；只动未派发的。
func (s *Store) Move(id, beforeID int) error {
	q, err := s.Queue()
	if err != nil {
		return err
	}
	var order []int
	for _, t := range q {
		if t.ID == id {
			continue
		}
		if beforeID > 0 && t.ID == beforeID {
			order = append(order, id)
		}
		order = append(order, t.ID)
	}
	if beforeID > 0 {
		found := false
		for _, x := range order {
			if x == beforeID {
				found = true
			}
		}
		if !found { // beforeID 不在队列里（可能已派发）：挪队尾
			order = append(order, id)
		}
	} else if beforeID == 0 {
		order = append([]int{id}, order...)
	} else {
		order = append(order, id) // beforeID<0：挪队尾
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for i, tid := range order {
		if _, err := tx.Exec(`UPDATE tasks SET prio=? WHERE id=?`, float64(i+1), tid); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ---- 窗口勾选 ----

type WindowPref struct {
	Key      string
	Label    string
	Included bool
}

// WindowRow 是给命令行列表用的持久窗口记录（不连 OpenClaw 库也能列出上次扫到的）。
type WindowRow struct {
	Key         string
	Label       string
	Model       string
	Provider    string
	LastStatus  string
	Schedulable bool
}

func (s *Store) WindowsAll() ([]WindowRow, error) {
	rows, err := s.db.Query(`SELECT key, IFNULL(label,''), IFNULL(model,''), IFNULL(provider,''),
		IFNULL(last_status,''), included FROM windows ORDER BY IFNULL(last_seen,0) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WindowRow{}
	for rows.Next() {
		var (
			r   WindowRow
			inc int
		)
		if err := rows.Scan(&r.Key, &r.Label, &r.Model, &r.Provider, &r.LastStatus, &inc); err != nil {
			return nil, err
		}
		// included=0 有两种：未勾选 和 本身不可派工。心跳/cron/子代理的键能直接认出来。
		r.Schedulable = !strings.Contains(r.Key, "heartbeat") &&
			!strings.Contains(r.Key, ":cron:") && !strings.Contains(r.Key, "subagent")
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpsertWindow 每次扫描刷新可见窗口清单；新窗口默认不纳入。
func (s *Store) UpsertWindow(key, label, status, model, provider string, schedulable bool) error {
	if !schedulable {
		// 心跳/定时任务/子代理窗口：强制不纳入，防「扰乱视听」
		_, err := s.db.Exec(`INSERT INTO windows(key,label,included,last_seen,last_status,model,provider)
			VALUES(?,?,0,strftime('%s','now')*1000,?,?,?)
			ON CONFLICT(key) DO UPDATE SET label=excluded.label, included=0,
			last_seen=excluded.last_seen, last_status=excluded.last_status,
			model=excluded.model, provider=excluded.provider`, key, label, status, model, provider)
		return err
	}
	_, err := s.db.Exec(`INSERT INTO windows(key,label,included,last_seen,last_status,model,provider)
		VALUES(?,?,0,strftime('%s','now')*1000,?,?,?)
		ON CONFLICT(key) DO UPDATE SET label=excluded.label,
		last_seen=excluded.last_seen, last_status=excluded.last_status,
		model=excluded.model, provider=excluded.provider`, key, label, status, model, provider)
	return err
}

func (s *Store) SetIncluded(key string, on bool) error {
	_, err := s.db.Exec(`UPDATE windows SET included=? WHERE key=?`, boolInt(on), key)
	return err
}

func (s *Store) Included() ([]WindowPref, error) {
	rows, err := s.db.Query(`SELECT key, IFNULL(label,''), included FROM windows WHERE included=1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WindowPref{}
	for rows.Next() {
		var w WindowPref
		var inc int
		rows.Scan(&w.Key, &w.Label, &inc)
		w.Included = inc == 1
		out = append(out, w)
	}
	return out, rows.Err()
}

// AutoIncludeNew 打开时：扫描遇到未记录过的可派工窗口自动勾选。
func (s *Store) AutoIncludeNew(seenKeys []string) error {
	v, _ := s.GetSetting("auto_include_new")
	if v != "1" {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, k := range seenKeys {
		if _, err := tx.Exec(`UPDATE windows SET included=1 WHERE key=? AND key NOT IN
			(SELECT key FROM windows WHERE included=1)`, k); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ---- 设置 ----

func (s *Store) GetSetting(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings(key,value) VALUES(?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// ---- 日志 ----

func (s *Store) logEvent(taskID int, kind, detail string) {
	s.db.Exec(`INSERT INTO events(at,task_id,kind,detail) VALUES(strftime('%s','now')*1000,?,?,?)`,
		taskID, kind, detail)
}

func (s *Store) Log(kind string, format string, args ...any) {
	d := format
	if len(args) > 0 {
		d = fmt.Sprintf(format, args...)
	}
	s.db.Exec(`INSERT INTO events(at,task_id,kind,detail) VALUES(strftime('%s','now')*1000,0,?,?)`, kind, d)
}

func (s *Store) RecentEvents(limit int) ([]string, error) {
	rows, err := s.db.Query(`SELECT datetime(at/1000,'unixepoch','localtime'), IFNULL(task_id,0), kind, IFNULL(detail,'')
		FROM events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var at, kind, detail string
		var tid int
		rows.Scan(&at, &tid, &kind, &detail)
		out = append(out, fmt.Sprintf("%s [%s] #%d %s", at, kind, tid, detail))
	}
	return out, rows.Err()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

var _ = strings.TrimSpace
var _ = taskIDFromPrio
